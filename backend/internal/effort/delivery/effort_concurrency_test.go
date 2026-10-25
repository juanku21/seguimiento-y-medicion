package delivery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Prueba de integración del tope diario ante altas simultáneas (T016; SC-005, FR-005): la misma
// cuenta envía a la vez POST /api/v1/stories/{storyId}/effort-entries contra la base smye_test para
// la misma fecha, repartidas en dos proyectos, y entre todas superan las 24 horas. Cada repetición
// usa una fecha distinta y sin horas cargadas para que el intercalado entre las transacciones ocurra
// de verdad. No lleva la etiqueta de un escenario de la spec porque verifica un criterio de éxito y
// no un escenario Given/When/Then.

// Cantidad de repeticiones, de altas simultáneas por repetición y horas de cada alta. Diez altas de
// 3 horas suman 30: caben exactamente 8 (24 horas) y sobran 2.
const (
	effortConcurrentRounds   = 5
	effortConcurrentRequests = 10
	effortConcurrentHours    = 3
	effortConcurrentAccepted = 8
)

// effortRequestsAtOnce ejecuta n solicitudes desde goroutines que esperan la misma señal de salida,
// para maximizar la simultaneidad, y devuelve las respuestas en el orden de los índices. Cada
// goroutine escribe solo en su posición del slice, así que no comparten estado mutable.
func effortRequestsAtOnce(n int, send func(i int) *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	recs := make([]*httptest.ResponseRecorder, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			recs[i] = send(i)
		})
	}
	close(start)
	wg.Wait()
	return recs
}

// Diez altas simultáneas de 3 horas de Ana para la misma fecha, alternadas entre una historia de cada
// uno de sus dos proyectos: en cada repetición exactamente 8 responden 201 y 2 responden 409
// daily_cap_exceeded, ninguna 500, y la suma de la fecha en la base es 24,00 (SC-005).
func TestEffortDailyCap_Concurrency(t *testing.T) {
	f := newEffortFixture(t)
	secondProject := f.env.createProjectInProgress(t, f.ownerToken, "Mesa de Ayuda")
	secondStory := f.env.createReadyStory(t, f.ownerToken, secondProject, "Alta de tickets")
	stories := []uuid.UUID{f.storyID, secondStory}

	// Cada repetición usa una fecha propia, entre el inicio del proyecto y hoy, sin horas cargadas.
	for round := range effortConcurrentRounds {
		entryDate := fmt.Sprintf("2026-10-%02d", 17+round)
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			before := f.env.effortEntryCount(t)

			recs := effortRequestsAtOnce(effortConcurrentRequests, func(i int) *httptest.ResponseRecorder {
				return f.env.postEffort(t, f.ownerToken, stories[i%len(stories)],
					effortBody(entryDate, effortActivity, effortConcurrentHours))
			})

			// Cada respuesta es un alta o un rechazo por el tope diario; cualquier otro código es un error.
			created, rejected := 0, 0
			for i, rec := range recs {
				switch rec.Code {
				case http.StatusCreated:
					created++
				case http.StatusConflict:
					var body effortConflictResponse
					if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "solicitud %d: el 409 no es JSON válido: %s", i+1, rec.Body.String()) {
						assert.Equal(t, "daily_cap_exceeded", body.Code, "solicitud %d: code del 409", i+1)
					}
					rejected++
				default:
					t.Errorf("solicitud %d: código %d inesperado; cuerpo: %s", i+1, rec.Code, rec.Body.String())
				}
			}
			assert.Equal(t, effortConcurrentAccepted, created, "deberían aceptarse exactamente 8 altas")
			assert.Equal(t, effortConcurrentRequests-effortConcurrentAccepted, rejected, "deberían rechazarse exactamente 2 altas")

			// En la base quedan solo las altas aceptadas y la fecha suma exactamente 24 horas.
			assert.Equal(t, before+int64(created), f.env.effortEntryCount(t), "las filas nuevas deberían ser las aceptadas")
			assert.Equal(t, int64(2400), f.env.userDateHundredths(t, f.ownerID, entryDate), "la fecha debería sumar 24,00 horas")
		})
	}
}
