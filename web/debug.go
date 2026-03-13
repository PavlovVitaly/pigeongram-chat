package web

import (
	"encoding/json"
	"net/http"
)

// DebugHandler возвращает статистику кэша
func DebugHandler(w http.ResponseWriter, r *http.Request) {
	if messageCache == nil {
		http.Error(w, "Cache not available", http.StatusNotFound)
		return
	}

	// Если у кэша есть метод GetStats, используем его
	if mc, ok := messageCache.(interface{ GetStats() interface{} }); ok {
		stats := mc.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// В main.go добавить маршрут:
// http.HandleFunc("/debug/cache", web.DebugHandler)
