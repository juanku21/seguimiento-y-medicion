package httpserver

import "github.com/gin-gonic/gin"

// LimitRequestBody devuelve el middleware global que acota el tamaño del cuerpo de cada solicitud,
// para que ningún manejador decodifique un JSON sin tope. Firma mínima de T094: la implementación
// con http.MaxBytesReader corresponde a T095.
func LimitRequestBody() gin.HandlerFunc {
	return func(*gin.Context) {}
}
