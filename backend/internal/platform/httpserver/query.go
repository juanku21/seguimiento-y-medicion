package httpserver

import (
	"github.com/gin-gonic/gin"
)

// RejectMalformedQuery devuelve el middleware global que rechaza con 400 la solicitud cuya query no
// se puede interpretar, para que un listado nunca ignore sus filtros sin aviso. Firma mínima de
// T104: por ahora deja pasar toda solicitud; la validación la agrega T105.
func RejectMalformedQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
