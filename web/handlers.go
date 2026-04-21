package web

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"pigeongram-chat/pkg/models"
	"pigeongram-chat/repository/cache"
	"pigeongram-chat/repository/postgres"
)

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

// LoginPage - страница входа
func LoginPage(w http.ResponseWriter, r *http.Request) {
	if getUserFromSession(r) != "" {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	errorMsg := r.URL.Query().Get("error")
	data := map[string]interface{}{
		"Error": errorMsg,
	}

	renderTemplate(w, "login.html", data)
}

// RegisterPage - страница регистрации
func RegisterPage(w http.ResponseWriter, r *http.Request) {
	if getUserFromSession(r) != "" {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	errorMsg := r.URL.Query().Get("error")
	data := map[string]interface{}{
		"Error": errorMsg,
	}

	renderTemplate(w, "register.html", data)
}

// ChatPage - страница чата
func ChatPage(w http.ResponseWriter, r *http.Request) {
	username := getUserFromSession(r)
	if username == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	log.Printf("👤 [CHAT] Пользователь %s открывает чат", username)

	// 👈 ВАЖНО: передаем просто строку, не map
	renderTemplate(w, "chat.html", username)
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

	// Validate уже использует bcrypt
	valid, err := userRepo.Validate(ctx, username, password)
	if err != nil || !valid {
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	// Получаем пользователя для создания сессии
	user, err := userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
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
		SameSite: http.SameSiteStrictMode,
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

	ctx := context.Background()

	// Проверяем существование
	existingUser, _ := userRepo.GetByUsername(ctx, username)
	if existingUser != nil {
		http.Redirect(w, r, "/register?error=exists", http.StatusSeeOther)
		return
	}

	// Создаем пользователя
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

	sessionID := generateSessionID()

	// Создаем сессию
	if createdUser != nil {
		sessionRepo.CreateUserSession(ctx, createdUser.ID, sessionID)
	}

	// Сохраняем в кэш
	if messageCache != nil {
		webUser := &models.User{Username: username, Password: password}
		messageCache.SetUser(ctx, username, webUser)

		webSession := &models.Session{Username: username}
		messageCache.SetSession(ctx, sessionID, webSession)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
		SameSite: http.SameSiteStrictMode,
	})

	log.Printf("✅ Новый пользователь зарегистрирован: %s", username)
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

// LogoutHandler - обрабатывает выход
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		ctx := context.Background()

		sessionRepo.Delete(ctx, cookie.Value)

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

// generateSessionID генерирует ID сессии
func generateSessionID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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
