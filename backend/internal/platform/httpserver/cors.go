// Package httpserver reúne la configuración HTTP transversal que comparte la aplicación, para que
// cmd/app solo componga dependencias (Principio III) y las pruebas ejerciten la misma configuración.
package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS devuelve el middleware que habilita las peticiones del navegador solo desde el origen del
// frontend. Firma mínima de la etapa RED de T085: replica la configuración vigente en cmd/app, que
// todavía no incluye PUT.
func CORS(allowedOrigin string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{allowedOrigin},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		MaxAge:       12 * time.Hour,
	})
}
