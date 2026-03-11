package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
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
	Username  string
	Text      string
	Timestamp time.Time
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
}

func InitMessageStore() {
	// Несколько тестовых сообщений
	messages = append(messages, Message{
		Username:  "test",
		Text:      "Добро пожаловать в чат!",
		Timestamp: time.Now(),
	})
}

// Вспомогательные функции
func generateSessionID() string {
	return time.Now().Format("20060102150405") + randomString(5)
}

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
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
	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	tmpl.Execute(w, nil)
}

func RegisterPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/register.html"))
	tmpl.Execute(w, nil)
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

	username := r.FormValue("username")
	password := r.FormValue("password")

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

func SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := getSessionUser(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	text := strings.TrimSpace(r.FormValue("message"))
	if text == "" {
		http.Error(w, "Empty message", http.StatusBadRequest)
		return
	}

	messagesMu.Lock()
	messages = append(messages, Message{
		Username:  username,
		Text:      text,
		Timestamp: time.Now(),
	})
	messagesMu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	username := getSessionUser(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	messagesMu.RLock()
	// Создаем копию для отправки
	messagesCopy := make([]Message, len(messages))
	copy(messagesCopy, messages)
	messagesMu.RUnlock()

	// Сортируем по времени (от старых к новым)
	sort.Slice(messagesCopy, func(i, j int) bool {
		return messagesCopy[i].Timestamp.Before(messagesCopy[j].Timestamp)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messagesCopy)
}
