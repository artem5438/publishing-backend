package api

import (
	"encoding/json"
	"net/http"
)

//  Singleton: текущий пользователь
// В лаб. 4 берём из сессии. Если сессии нет — fallback на константу 1 (для SSR).

func getCreatorID(r *http.Request) uint {
	if id, ok := GetUserIDFromCtx(r); ok {
		return id
	}
	return 1 // fallback для SSR-маршрутов
}

//  JSON-хелперы

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
