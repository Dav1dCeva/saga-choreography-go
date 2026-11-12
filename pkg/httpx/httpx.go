// Package httpx contiene handlers HTTP compartidos por los servicios.
package httpx

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// Health expone /health comprobando la conexión a PostgreSQL.
func Health(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := "ok"
		code := http.StatusOK
		if err := db.PingContext(ctx); err != nil {
			status = "error"
			code = http.StatusServiceUnavailable
		}
		WriteJSON(w, code, map[string]string{"status": status})
	}
}

// WriteJSON escribe una respuesta JSON con el código indicado.
func WriteJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError devuelve un error JSON uniforme.
func WriteError(w http.ResponseWriter, code int, message string) {
	WriteJSON(w, code, map[string]string{"error": message})
}
