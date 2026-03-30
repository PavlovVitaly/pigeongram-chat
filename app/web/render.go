package web

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// renderTemplate рендерит HTML шаблон с данными
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	// Создаем полный путь к шаблону
	templatePath := filepath.Join("web", "templates", tmpl)

	// Парсим шаблон
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("❌ Ошибка парсинга шаблона %s: %v", tmpl, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Выполняем шаблон
	if err := t.Execute(w, data); err != nil {
		log.Printf("❌ Ошибка выполнения шаблона %s: %v", tmpl, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// renderTemplateWithLayout рендерит шаблон с базовым layout (для будущего использования)
func renderTemplateWithLayout(w http.ResponseWriter, tmpl string, data interface{}) {
	layoutPath := filepath.Join("web", "templates", "layout.html")
	templatePath := filepath.Join("web", "templates", tmpl)

	// Парсим layout и шаблон вместе
	t, err := template.ParseFiles(layoutPath, templatePath)
	if err != nil {
		log.Printf("❌ Ошибка парсинга шаблона с layout: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Выполняем шаблон (предполагаем, что в layout есть {{template "content" .}})
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("❌ Ошибка выполнения шаблона с layout: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
