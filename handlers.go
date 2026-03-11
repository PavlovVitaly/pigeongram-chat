package main

import (
	"html/template"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Структуры данных
type User struct {
	Username string
	Password string
}

type Message struct {
	Username  string    `json:"username"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	Username string
}

// Хранилища
var (
	users      = make(map[string]User)
	messages   = make([]Message, 0)
	sessions   = make(map[string]Session)
	usersMu    sync.RWMutex
	messagesMu sync.RWMutex
	sessionsMu sync.RWMutex
)

// Инициализация хранилищ
func InitUserStore() {
	// Добавим тестового пользователя для удобства
	users["test"] = User{Username: "test", Password: "test"}
	users["admin"] = User{Username: "admin", Password: "admin"}
}

func InitMessageStore() {
	// Несколько тестовых сообщений
	messages = append(messages, Message{
		Username:  "test",
		Text:      "Добро пожаловать в чат с WebSocket!",
		Timestamp: time.Now(),
	})
	messages = append(messages, Message{
		Username:  "admin",
		Text:      "Теперь сообщения приходят мгновенно!",
		Timestamp: time.Now().Add(-time.Minute),
	})
}

// Вспомогательные функции
func generateSessionID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func getSessionUser(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}

	sessionsMu.RLock()
	session, exists := sessions[cookie.Value]
	sessionsMu.RUnlock()

	if !exists {
		return ""
	}
	return session.Username
}

// Обработчики страниц
func LoginPage(w http.ResponseWriter, r *http.Request) {
	// Если пользователь уже залогинен, перенаправляем в чат
	if username := getSessionUser(r); username != "" {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	// Получаем параметр ошибки из URL
	errorMsg := r.URL.Query().Get("error")

	tmpl := template.Must(template.ParseFiles("templates/login.html"))

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
	if username := getSessionUser(r); username != "" {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	// Получаем параметр ошибки из URL
	errorMsg := r.URL.Query().Get("error")

	tmpl := template.Must(template.ParseFiles("templates/register.html"))

	// Передаем данные в шаблон
	data := struct {
		Error string
	}{
		Error: errorMsg,
	}

	tmpl.Execute(w, data)
}

func ChatPage(w http.ResponseWriter, r *http.Request) {
	username := getSessionUser(r)
	if username == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/chat.html"))
	tmpl.Execute(w, username)
}

// Обработчики действий
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

	usersMu.RLock()
	user, exists := users[username]
	usersMu.RUnlock()

	if !exists || user.Password != password {
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	sessionID := generateSessionID()
	sessionsMu.Lock()
	sessions[sessionID] = Session{Username: username}
	sessionsMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600 * 24, // 24 часа
	})

	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	// Валидация
	if username == "" || password == "" {
		http.Redirect(w, r, "/register?error=empty", http.StatusSeeOther)
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

	if password != confirmPassword {
		http.Redirect(w, r, "/register?error=password_mismatch", http.StatusSeeOther)
		return
	}

	usersMu.Lock()
	if _, exists := users[username]; exists {
		usersMu.Unlock()
		http.Redirect(w, r, "/register?error=user_exists", http.StatusSeeOther)
		return
	}

	users[username] = User{Username: username, Password: password}
	usersMu.Unlock()

	// Автоматический логин после регистрации
	sessionID := generateSessionID()
	sessionsMu.Lock()
	sessions[sessionID] = Session{Username: username}
	sessionsMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600 * 24,
	})

	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

// Добавьте эту функцию в файл handlers.go

// LogoutHandler - обработчик выхода из системы
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем сессию из cookie
	cookie, err := r.Cookie("session_id")
	if err == nil {
		// Удаляем сессию из хранилища
		sessionsMu.Lock()
		delete(sessions, cookie.Value)
		sessionsMu.Unlock()
	}

	// Удаляем cookie на стороне клиента
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,              // Мгновенное истечение
		Expires:  time.Unix(0, 0), // Устанавливаем дату в прошлом
	})

	// Перенаправляем на страницу входа
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// AuthMiddleware - проверяет авторизацию пользователя
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := getSessionUser(r)
		if username == "" {
			// Если пользователь не авторизован, перенаправляем на страницу входа
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
