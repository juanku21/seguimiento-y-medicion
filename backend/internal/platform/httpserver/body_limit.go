package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// maxRequestBodyBytes es el tope de 1 MiB para el cuerpo de cualquier solicitud.
const maxRequestBodyBytes = 1 << 20

// LimitRequestBody devuelve el middleware global que acota el tamaño del cuerpo de cada solicitud,
// para que ningún manejador decodifique un JSON sin tope. http.MaxBytesReader corta la lectura al
// superar el límite, de modo que ShouldBindJSON falla con un error en lugar de consumir el cuerpo
// completo.
func LimitRequestBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		c.Next()
	}
}
