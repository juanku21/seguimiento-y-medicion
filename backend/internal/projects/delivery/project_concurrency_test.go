package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// projectNameGroup es una fila del conteo de proyectos agrupado por propietario y nombre
// normalizado, la clave del índice único compuesto (research.md sección 2).
type projectNameGroup struct {
	OwnerID        uuid.UUID
	NameNormalized string
	Total          int64
}

// Escenario: Dos creaciones simultáneas del mismo nombre por el mismo propietario (002/US1)
// Diez solicitudes de creación simultáneas con el mismo nombre y el mismo propietario dejan
// exactamente un proyecto con su única membresía de propietario, y las nueve restantes reciben 409
// por nombre repetido. Se verifica contando las filas agrupadas por (owner_id, name_normalized)
// (FR-011, SC-007).
func TestUS1_CreacionesSimultaneas(t *testing.T) {
	env := newProjectsTestEnv(t)
	userID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	const requests = 10

	// Todas las goroutines esperan la misma señal de salida para maximizar la simultaneidad; cada
	// una escribe en su propio recorder, de modo que no comparten estado mutable.
	start := make(chan struct{})
	recorders := make([]*httptest.ResponseRecorder, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			<-start
			recorders[i] = env.postProject(t, token, validProjectInputBody())
		})
	}
	close(start)
	wg.Wait()

	// Se clasifican las respuestas: una sola creación y el resto rechazadas por nombre repetido con
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
			assert.Equal(t, projectNameTakenMessage, got.Error)
		default:
			t.Errorf("estado inesperado %d; cuerpo: %s", rec.Code, rec.Body.String())
		}
	}
	require.Len(t, created, 1, "respuestas: %d con 201 y %d con 409, se esperaba 1 y %d", len(created), conflicts, requests-1)
	require.Equal(t, requests-1, conflicts, "se esperaban %d respuestas 409", requests-1)
	project := requireCreatedProject(t, created[0])

	// Se cuentan las filas agrupadas por la clave de unicidad: debe haber un único grupo, el del
	// propietario con el nombre normalizado, y con una sola fila.
	var groups []projectNameGroup
	require.NoError(t, env.db.Model(&domain.Project{}).
		Select("owner_id, name_normalized, COUNT(*) AS total").
		Group("owner_id, name_normalized").
		Scan(&groups).Error, "no se pudieron agrupar los proyectos")
	require.Len(t, groups, 1, "grupos (owner_id, name_normalized): %+v", groups)
	assert.Equal(t, userID, groups[0].OwnerID)
	assert.Equal(t, "portal de clientes", groups[0].NameNormalized)
	assert.Equal(t, int64(1), groups[0].Total, "hay %d proyectos con el mismo nombre para el propietario", groups[0].Total)

	// Los intentos perdedores no dejan membresías huérfanas: solo existe la del propietario en el
	// proyecto creado.
	assert.Equal(t, int64(1), countMemberRows(t, env.db, ""))
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ? AND user_id = ?", project.ID, userID))
}
