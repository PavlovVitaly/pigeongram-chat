package web

import (
	"context"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"pigeongram/pkg/models" // DTO
	// сущности БД
	"pigeongram/repository/cache"
	"pigeongram/repository/postgres"
)

// Хранилища
var (
	userRepo     *postgres.UserRepository
	msgRepo      *postgres.MessageRepository
	sessionRepo  *postgres.SessionRepository
	messageCache cache.Cache
)

// InitStores инициализирует репозитории и кэш
func InitStores(ur *postgres.UserRepository, mr *postgres.MessageRepository, sr *postgres.SessionRepository, mc cache.Cache) {
	userRepo = ur
	msgRepo = mr
	sessionRepo = sr
	messageCache = mc
	log.Println("📦 Репозитории и кэш инициализированы")
}

// generateSessionID генерирует ID сессии
func generateSessionID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// LoginPage - страница входа
func LoginPage(w http.ResponseWriter, r *http.Request) {
	// Если пользователь уже залогинен, перенаправляем в чат
	if getUserFromSession(r) != "" {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	// Получаем параметр ошибки из URL
	errorMsg := r.URL.Query().Get("error")

	tmpl := template.Must(template.ParseFiles("web/templates/login.html"))

	// Передаем данные в шаблон
	data := struct {
		Error string
	}{
		Error: errorMsg,
	}

	tmpl.Execute(w, data)
}

func RegisterPage(w http.ResponseWriter, r *http.Request) {
	// Если пользователь уже залогинен, перенаправляем в чат
	if getUserFromSession(r) != "" {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	// Получаем параметр ошибки из URL
	errorMsg := r.URL.Query().Get("error")

	tmpl := template.Must(template.ParseFiles("web/templates/register.html"))

	// Передаем данные в шаблон
	data := struct {
		Error string
	}{
		Error: errorMsg,
	}

	tmpl.Execute(w, data)
}

// ChatPage - страница чата
func ChatPage(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("web/templates/chat.html"))
	tmpl.Execute(w, username)
}

// LoginHandler - обрабатывает вход
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Redirect(w, r, "/?error=empty", http.StatusSeeOther)
		return
	}

	ctx := context.Background()

	// Проверяем в БД
	user, err := userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil || user.Password != password {
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	// Сохраняем в кэш
	if messageCache != nil {
		webUser := &models.User{Username: user.Username, Password: user.Password}
		messageCache.SetUser(ctx, username, webUser)
	}

	sessionID := generateSessionID()

	// Создаем сессию в БД
	err = sessionRepo.CreateUserSession(ctx, user.ID, sessionID)
	if err != nil {
		log.Printf("❌ Ошибка создания сессии: %v", err)
		http.Redirect(w, r, "/?error=server", http.StatusSeeOther)
		return
	}

	// Сохраняем сессию в кэш
	if messageCache != nil {
		webSession := &models.Session{Username: username}
		messageCache.SetSession(ctx, sessionID, webSession)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	log.Printf("✅ Пользователь %s вошел в систему", username)
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

// RegisterHandler - обрабатывает регистрацию
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	if username == "" || password == "" || password != confirm {
		http.Redirect(w, r, "/register?error=1", http.StatusSeeOther)
		return
	}

	if len(username) < 3 || len(username) > 20 {
		http.Redirect(w, r, "/register?error=username_length", http.StatusSeeOther)
		return
	}

	if len(password) < 4 {
		http.Redirect(w, r, "/register?error=password_length", http.StatusSeeOther)
		return
	}

	ctx := context.Background()

	// Проверяем существование в БД
	existingUser, _ := userRepo.GetByUsername(ctx, username)
	if existingUser != nil {
		http.Redirect(w, r, "/register?error=exists", http.StatusSeeOther)
		return
	}

	// Создаем в БД
	newUser := &models.User{
		Username: username,
		Password: password,
	}

	err := userRepo.Create(ctx, newUser)
	if err != nil {
		log.Printf("❌ Ошибка создания пользователя: %v", err)
		http.Redirect(w, r, "/register?error=server", http.StatusSeeOther)
		return
	}

	// Получаем созданного пользователя с ID
	createdUser, _ := userRepo.GetByUsername(ctx, username)

	// Сохраняем в кэш
	if messageCache != nil {
		webUser := &models.User{Username: username, Password: password}
		messageCache.SetUser(ctx, username, webUser)
	}

	sessionID := generateSessionID()

	// Создаем сессию в БД
	err = sessionRepo.CreateUserSession(ctx, createdUser.ID, sessionID)
	if err != nil {
		log.Printf("❌ Ошибка создания сессии: %v", err)
	}

	// Сохраняем сессию в кэш
	if messageCache != nil {
		webSession := &models.Session{Username: username}
		messageCache.SetSession(ctx, sessionID, webSession)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	log.Printf("✅ Новый пользователь зарегистрирован: %s", username)
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

// LogoutHandler - обрабатывает выход
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		ctx := context.Background()

		// Удаляем из БД
		sessionRepo.Delete(ctx, cookie.Value)

		// Удаляем из кэша
		if messageCache != nil {
			messageCache.InvalidateSession(ctx, cookie.Value)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	log.Println("👋 Пользователь вышел из системы")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// AuthMiddleware - проверяет авторизацию пользователя
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := getUserFromSession(r)
		if username == "" {
			// Если пользователь не авторизован, перенаправляем на страницу входа
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// getUserFromSession получает пользователя из сессии
func getUserFromSession(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}

	ctx := context.Background()

	// Пробуем получить из кэша
	if messageCache != nil {
		cachedSession, err := messageCache.GetSession(ctx, cookie.Value)
		if err == nil && cachedSession != nil {
			return cachedSession.Username
		}
	}

	// Если нет в кэше, грузим из БД
	session, err := sessionRepo.GetByID(ctx, cookie.Value)
	if err != nil || session == nil || session.IsExpired() {
		return ""
	}

	// Сохраняем в кэш
	if messageCache != nil {
		webSession := &models.Session{Username: session.User.Username}
		messageCache.SetSession(ctx, cookie.Value, webSession)
	}

	return session.User.Username
}
