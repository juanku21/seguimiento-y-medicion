package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Escenario: Dos altas simultáneas del mismo integrante (002/US3)
// Diez altas simultáneas de Bruno en el mismo proyecto, enviadas por el propietario, registran una
// sola membresía: una solicitud recibe 201 y las nueve restantes reciben 409 por integrante
// duplicado. Se verifica contando las filas de project_members del par (proyecto, Bruno), la clave
// del índice único compuesto (FR-023, SC-008).
func TestUS3_AltasSimultaneas(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)
	const requests = 10

	// Todas las goroutines esperan la misma señal de salida para maximizar la simultaneidad; cada
	// una escribe en su propio recorder, de modo que no comparten estado mutable.
	start := make(chan struct{})
	recorders := make([]*httptest.ResponseRecorder, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			<-start
			recorders[i] = env.postMember(t, ownerToken, projectID, "bruno@example.com")
		})
	}
	close(start)
	wg.Wait()

	// Se clasifican las respuestas: un solo alta y el resto rechazadas por integrante duplicado con
	// el texto exacto del contrato.
	var created []*httptest.ResponseRecorder
	conflicts := 0
	for _, rec := range recorders {
		switch rec.Code {
		case http.StatusCreated:
			created = append(created, rec)
		case http.StatusConflict:
			conflicts++
			var got projectErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
			assert.Equal(t, memberAlreadyMemberMessage, got.Error)
		default:
			t.Errorf("estado inesperado %d; cuerpo: %s", rec.Code, rec.Body.String())
		}
	}
	require.Len(t, created, 1, "respuestas: %d con 201 y %d con 409, se esperaba 1 y %d", len(created), conflicts, requests-1)
	require.Equal(t, requests-1, conflicts, "se esperaban %d respuestas 409", requests-1)
	assert.Equal(t, brunoID.String(), requireCreatedMember(t, created[0]).UserID)

	// En la base queda una sola membresía de Bruno en el proyecto y, en total, solo las dos
	// membresías esperadas: la del propietario y la de Bruno.
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ? AND user_id = ?", projectID, brunoID))
	assert.Len(t, memberRowsOf(t, env.db, projectID), 2, "el proyecto debería tener exactamente dos integrantes")
}
