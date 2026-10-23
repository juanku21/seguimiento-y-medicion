package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Prueba del middleware que rechaza las queries mal formadas (T104, hallazgo de seguridad: query mal
// formada que ignora los filtros sin aviso, detectado en 004/US6). El router usa solo el middleware y
// una ruta que responde 200 si la solicitud llega al manejador, para distinguir el rechazo del paso.

// queryProbePath es la ruta de prueba que responde 200 cuando la solicitud llega al manejador.
const queryProbePath = "/probe"

// newQueryProbeRouter compone un router con RejectMalformedQuery y la ruta de prueba, que marca en
// reached si el manejador llegó a ejecutarse.
func newQueryProbeRouter(reached *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RejectMalformedQuery())
	router.GET(queryProbePath, func(c *gin.Context) {
		*reached = true
		c.Status(http.StatusOK)
	})
	return router
}

// repeatedStatusQuery arma una query con el parámetro status=closed repetido n veces.
func repeatedStatusQuery(n int) string {
	return strings.TrimSuffix(strings.Repeat("status=closed&", n), "&")
}

// Sin etiqueta de escenario: cada query que url.ParseQuery no puede interpretar (más de 10000
// parámetros, un escape inválido o un separador ";") se rechaza con 400 y un mensaje en el campo
// error, sin llegar al manejador; las queries válidas, vacías o con repeticiones por debajo del tope
// pasan sin cambios.
func TestRejectMalformedQuery(t *testing.T) {
	// Queries que no se pueden interpretar: el middleware corta con 400.
	malformed := []struct {
		name     string
		rawQuery string
	}{
		{name: "mas de 10000 parametros", rawQuery: repeatedStatusQuery(10001)},
		{name: "escape invalido", rawQuery: "status=%zz"},
		{name: "separador punto y coma", rawQuery: "status=closed;x"},
	}
	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			router := newQueryProbeRouter(&reached)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, queryProbePath+"?"+tc.rawQuery, nil))

			require.Equal(t, http.StatusBadRequest, rec.Code, "la query mal formada debería rechazarse con 400")
			assert.False(t, reached, "la solicitud no debería llegar al manejador")
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 400 no es JSON válido: %s", rec.Body.String())
			message, _ := body["error"].(string)
			assert.NotEmpty(t, strings.TrimSpace(message), "el 400 debería explicar el motivo en el campo error: %s", rec.Body.String())
		})
	}

	// Queries válidas: el middleware deja pasar la solicitud al manejador.
	valid := []struct {
		name     string
		rawQuery string
	}{
		{name: "sin query", rawQuery: ""},
		{name: "filtros validos", rawQuery: "status=closed&status=active"},
		{name: "10000 parametros", rawQuery: repeatedStatusQuery(10000)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			router := newQueryProbeRouter(&reached)
			target := queryProbePath
			if tc.rawQuery != "" {
				target += "?" + tc.rawQuery
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

			assert.Equal(t, http.StatusOK, rec.Code, "la query válida debería llegar al manejador")
			assert.True(t, reached, "la solicitud debería llegar al manejador")
		})
	}
}
