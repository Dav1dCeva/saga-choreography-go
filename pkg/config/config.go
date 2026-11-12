// Package config centraliza la lectura de variables de entorno.
package config

import (
	"os"
	"strconv"
	"strings"
)

// String devuelve el valor de la variable o el valor por defecto.
func String(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// Bool interpreta 1, true, yes o on como verdadero.
func Bool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// Float interpreta un número decimal o devuelve el valor por defecto.
func Float(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
