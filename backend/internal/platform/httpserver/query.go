package httpserver

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// malformedQueryMessage es el mensaje del 400 ante una query que no se puede interpretar.
const malformedQueryMessage = "La consulta de la URL está mal formada: revisá que no supere los 10000 parámetros, que los caracteres especiales estén bien codificados y que los parámetros se separen con &, no con ;."

// RejectMalformedQuery devuelve el middleware global que rechaza con 400 la solicitud cuya query no
// se puede interpretar, para que un listado nunca ignore sus filtros sin aviso. url.ParseQuery falla
// con más de 10000 parámetros, con un escape inválido o con un separador ";", los mismos casos en que
// gin descartaría la query en silencio; el cuerpo tiene la forma de ErrorResponse.
func RejectMalformedQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := url.ParseQuery(c.Request.URL.RawQuery); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": malformedQueryMessage})
			return
		}
		c.Next()
	}
}
