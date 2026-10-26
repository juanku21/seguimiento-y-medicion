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

// Prueba de integración del tope diario ante una modificación y altas simultáneas (T045; FR-024,
// SC-005): con 18 horas ya cargadas en la fecha, de las cuales 2 son un registro propio sin sprint, la
// misma cuenta envía a la vez PATCH /api/v1/effort-entries/{entryId} que lleva ese registro de 2 a 6
// horas y tres altas de 2 horas. Entre todos piden 28 horas: cualquier orden serial termina en
// exactamente 24, rechazando por el tope lo que sobra. Tampoco lleva la etiqueta de un escenario de la
// spec porque verifica un criterio de éxito.

// Horas ya cargadas en la fecha además del registro a modificar, horas de ese registro antes y
// después de la modificación, y cantidad y horas de las altas simultáneas.
const (
	effortMixedSeededHundredths = 1600
	effortMixedEntryHundredths  = 200
	effortMixedPatchedHours     = 6
	effortMixedPosts            = 3
	effortMixedPostHours        = 2
)

// En cada repetición, la modificación de 2 a 6 horas y las tres altas de 2 horas compiten por las 6
// horas libres de la fecha: ninguna respuesta es 500, los rechazos son 409 daily_cap_exceeded, la
// fecha nunca supera 24,00 y lo guardado coincide con lo que las respuestas informaron como aceptado.
func TestEffortDailyCap_ConcurrencyMixed(t *testing.T) {
	f := newEffortFixture(t)

	// Cada repetición usa una fecha propia, entre el inicio del proyecto y hoy, con 18 horas cargadas.
	for round := range effortConcurrentRounds {
		entryDate := fmt.Sprintf("2026-10-%02d", 17+round)
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			f.env.seedEffortEntry(t, f.storyID, f.ownerID, entryDate, effortMixedSeededHundredths, nil)
			entryID := f.env.seedStoryEntry(t, f.storyID, effortSeed{
				userID: f.ownerID, entryDate: entryDate, hours: effortMixedEntryHundredths,
			}).String()
			before := f.env.effortEntryCount(t)

			// La solicitud 0 es la modificación; las siguientes, las altas.
			recs := effortRequestsAtOnce(1+effortMixedPosts, func(i int) *httptest.ResponseRecorder {
				if i == 0 {
					return f.env.patchEffort(t, f.ownerToken, entryID, map[string]any{"hours": effortMixedPatchedHours})
				}
				return f.env.postEffort(t, f.ownerToken, f.storyID, effortBody(entryDate, effortActivity, effortMixedPostHours))
			})

			// Cada respuesta es una aceptación (200 la modificación, 201 un alta) o un rechazo por el tope
			// diario; cualquier otro código, incluido el 500, es un error.
			patched, created := false, 0
			for i, rec := range recs {
				accepted := http.StatusCreated
				if i == 0 {
					accepted = http.StatusOK
				}
				switch rec.Code {
				case accepted:
					if i == 0 {
						patched = true
					} else {
						created++
					}
				case http.StatusConflict:
					var body effortConflictResponse
					if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "solicitud %d: el 409 no es JSON válido: %s", i, rec.Body.String()) {
						assert.Equal(t, "daily_cap_exceeded", body.Code, "solicitud %d: code del 409", i)
					}
				default:
					t.Errorf("solicitud %d: código %d inesperado; cuerpo: %s", i, rec.Code, rec.Body.String())
				}
			}

			// Lo guardado coincide con lo aceptado: el registro tiene 6 horas solo si la modificación se
			// aceptó, y hay una fila nueva por alta aceptada.
			wantEntry, wantTotal := effortMixedEntryHundredths, int64(effortMixedSeededHundredths+effortMixedEntryHundredths)
			if patched {
				wantEntry = effortMixedPatchedHours * 100
			}
			wantTotal += int64(wantEntry-effortMixedEntryHundredths) + int64(created*effortMixedPostHours*100)
			assert.Equal(t, wantEntry, f.env.storedEntry(t, entryID).HoursHundredths, "horas del registro modificado")
			assert.Equal(t, before+int64(created), f.env.effortEntryCount(t), "las filas nuevas deberían ser las altas aceptadas")

			// La fecha nunca supera 24 horas y, como todo orden serial agota las 6 horas libres, queda en
			// exactamente 24,00.
			total := f.env.userDateHundredths(t, f.ownerID, entryDate)
			assert.Equal(t, wantTotal, total, "la suma de la fecha debería reflejar lo aceptado")
			assert.LessOrEqual(t, total, int64(2400), "la fecha no debe superar 24,00 horas")
			assert.Equal(t, int64(2400), total, "la fecha debería sumar 24,00 horas")
		})
	}
}
