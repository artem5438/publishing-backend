package api

import (
	"encoding/json"
	"net/http"
)

// ─── Singleton: текущий пользователь (константа до лаб. 4) ──────────────────

const creatorIDConst uint = 1

func getCreatorID() uint {
	return creatorIDConst
}

// ─── JSON-хелперы ─────────────────────────────────────────────────────────────

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
