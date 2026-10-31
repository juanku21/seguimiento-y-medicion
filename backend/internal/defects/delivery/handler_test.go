package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// Pruebas de integración de punta a punta del registro de defectos (T011): atraviesan el router de
// Gin compuesto como en cmd/app (testenv_test.go), el middleware de sesión de la 001,
// RequireMembership de la 002, el servicio y el repositorio de defectos contra la base smye_test
// (Principios II y V). Cubren POST /api/v1/projects/{projectId}/defects con un caso por escenario de
// spec.md; los "Dado" de proyectos, historias y sprints se preparan con los endpoints reales. En cada
// rechazo se verifica que defects no cambió y que ever_had_defects de la historia sigue en false.
//
// TestUS1_CuatroSeveridades y TestUS1_SeveridadInvalida ya existen como pruebas unitarias en
// validation_test.go (mismo paquete), así que sus versiones de punta a punta llevan el sufijo
// "Registro" y conservan el comentario de escenario 007/US1.

// Textos exactos que fijan el contrato y tasks.md (T011, T020) para las respuestas del alta.
const (
	defectInvalidFieldsError = "Hay campos inválidos en la solicitud."
	defectMsgStoryNotFound   = "La historia no existe."
	defectMsgSprintNotFound  = "El sprint no existe."
	defectMsgReadOnly        = "El proyecto está finalizado y sus defectos son de solo lectura."
	defectMsgNoDetection     = "Todavía no hay ningún sprint Activo ni Cerrado que pueda ser sprint de detección."
	defectMsgSprintPlanned   = "Un defecto solo se detecta en un sprint Activo o Cerrado."
	defectMsgDescRequired    = "La descripción es obligatoria."
	defectMsgDescTooLong     = "La descripción admite como máximo 2000 caracteres."
	defectMsgSeverityInvalid = "La severidad debe ser Crítica, Alta, Media o Baja."
	defectMsgStoryRequired   = "La historia relacionada es obligatoria."
	defectMsgSprintRequired  = "El sprint de detección es obligatorio."
)

// defectDescription es la descripción del caso normal de la spec (US1, escenario 1).
const defectDescription = "El total del reporte no suma los descuentos"

// defectUserRef refleja UserRef del contrato.
type defectUserRef struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
}

// defectStoryRef refleja StoryRef del contrato.
type defectStoryRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// defectSprintRef refleja SprintRef del contrato.
type defectSprintRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartDate string `json:"startDate"`
}

// defectResponse refleja Defect del contrato; Raw guarda el cuerpo para distinguir resolvedSprint:
// null de un campo ausente.
type defectResponse struct {
	ID              string                     `json:"id"`
	ProjectID       string                     `json:"projectId"`
	Description     string                     `json:"description"`
	Severity        string                     `json:"severity"`
	Status          string                     `json:"status"`
	Story           defectStoryRef             `json:"story"`
	DetectedSprint  defectSprintRef            `json:"detectedSprint"`
	ResolvedSprint  *defectSprintRef           `json:"resolvedSprint"`
	CreatedBy       defectUserRef              `json:"createdBy"`
	UpdatedBy       defectUserRef              `json:"updatedBy"`
	StatusChangedAt string                     `json:"statusChangedAt"`
	StatusChangedBy defectUserRef              `json:"statusChangedBy"`
	CreatedAt       string                     `json:"createdAt"`
	UpdatedAt       string                     `json:"updatedAt"`
	Raw             map[string]json.RawMessage `json:"-"`
}

// defectConflictResponse refleja DefectConflictResponse del contrato.
type defectConflictResponse struct {
	Code            string          `json:"code"`
	Error           string          `json:"error"`
	CurrentStatus   json.RawMessage `json:"currentStatus"`
	AllowedStatuses []string        `json:"allowedStatuses"`
}

// defectValidationResponse refleja ValidationErrorResponse del contrato.
type defectValidationResponse struct {
	Error  string                    `json:"error"`
	Fields []authdelivery.FieldError `json:"fields"`
}

// requireDefectCreated exige un 201 cuyo cuerpo sea el defecto creado y lo devuelve decodificado.
func requireDefectCreated(t *testing.T, rec *httptest.ResponseRecorder) defectResponse {
	t.Helper()
	var defect defectResponse
	defectRequireStatus(t, rec, http.StatusCreated, &defect)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &defect.Raw), "el cuerpo del 201 no es un objeto JSON")
	return defect
}

// requireDefectFieldRejected exige un 400 de entrada inválida que informe el campo con el mensaje
// indicado.
func requireDefectFieldRejected(t *testing.T, rec *httptest.ResponseRecorder, field, message string) {
	t.Helper()
	var body defectValidationResponse
	defectRequireStatus(t, rec, http.StatusBadRequest, &body)
	assert.Equal(t, defectInvalidFieldsError, body.Error)
	for _, f := range body.Fields {
		if f.Field == field {
			assert.Equal(t, message, f.Message, "mensaje del campo %s", field)
			return
		}
	}
	t.Fatalf("el 400 no informa el campo %q; cuerpo: %s", field, rec.Body.String())
}

// requireDefectConflict exige un 409 con el código indicado y lo devuelve decodificado.
func requireDefectConflict(t *testing.T, rec *httptest.ResponseRecorder, code string) defectConflictResponse {
	t.Helper()
	var body defectConflictResponse
	defectRequireStatus(t, rec, http.StatusConflict, &body)
	assert.Equal(t, code, body.Code, "code del 409")
	assert.NotEmpty(t, body.Error, "el 409 debe llevar un mensaje para la persona")
	return body
}

// defectFixture es el "Dado" común de US1: Ana Pérez es propietaria de un proyecto En curso con un
// sprint Activo y una historia del backlog.
type defectFixture struct {
	env        *defectTestEnv
	ownerID    uuid.UUID
	ownerToken string
	projectID  uuid.UUID
	storyID    uuid.UUID
	sprintID   uuid.UUID
}

// newDefectFixtureWithoutSprint arma el "Dado" sin ningún sprint, para las pruebas que necesitan
// crear antes uno Cerrado (la 004 admite un solo sprint Activo a la vez).
func newDefectFixtureWithoutSprint(t *testing.T) defectFixture {
	t.Helper()
	env := newDefectTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProjectInProgress(t, ownerToken, "Portal de Reportes")
	storyID := env.createStory(t, ownerToken, projectID, "Reporte de ventas mensual")
	return defectFixture{env: env, ownerID: ownerID, ownerToken: ownerToken, projectID: projectID, storyID: storyID}
}

// newDefectFixture compone el entorno y arma el "Dado" común con los endpoints reales, incluido el
// sprint Activo.
func newDefectFixture(t *testing.T) defectFixture {
	t.Helper()
	f := newDefectFixtureWithoutSprint(t)
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	return f
}

// body arma el cuerpo de un alta válida sobre la historia y el sprint indicados; cada prueba quita o
// modifica los campos que le interesan.
func (f defectFixture) body(storyID, sprintID uuid.UUID) map[string]any {
	return map[string]any{
		"description":      defectDescription,
		"severity":         "high",
		"storyId":          storyID.String(),
		"detectedSprintId": sprintID.String(),
	}
}

// post envía el alta de Ana en su proyecto con el cuerpo recibido.
func (f defectFixture) post(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return f.env.postDefect(t, f.ownerToken, f.projectID, body)
}

// requireNothingCreated verifica que la cantidad de filas de defects sigue siendo la previa y que la
// historia sigue sin la marca ever_had_defects.
func (f defectFixture) requireNothingCreated(t *testing.T, before int64) {
	t.Helper()
	assert.Equal(t, before, f.env.defectCount(t), "un rechazo no debe crear defectos")
	assert.False(t, f.env.everHadDefects(t, f.storyID), "un rechazo no debe marcar ever_had_defects")
}

// requireNotFoundLikeUnknown exige el 404 indicado con el cuerpo exacto y idéntico byte a byte al de
// un UUID inexistente en el mismo campo.
func requireNotFoundLikeUnknown(t *testing.T, rec, unknown *httptest.ResponseRecorder, message string) {
	t.Helper()
	assert.Equal(t, http.StatusNotFound, unknown.Code, "precondición: el UUID inexistente responde 404")
	assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+message+`"}`, rec.Body.String())
	assert.Equal(t, unknown.Body.String(), rec.Body.String(), "el 404 debe ser idéntico byte a byte al de un UUID inexistente")
}

// Escenario: Dado un integrante de un proyecto En curso con un sprint Activo y una historia del backlog, cuando registra un defecto con severidad Alta, la descripción "El total del reporte no suma los descuentos" y el sprint Activo como sprint de detección, entonces el defecto queda creado en estado Abierto, sin sprint de resolución y atribuido a quien lo registró (007/US1)
func TestUS1_RegistroEnSprintActivo(t *testing.T) {
	f := newDefectFixture(t)

	defect := requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))

	// Nace Abierto, con los datos ingresados y sin sprint de resolución.
	assert.Equal(t, "open", defect.Status)
	assert.Equal(t, defectDescription, defect.Description)
	assert.Equal(t, "high", defect.Severity)
	assert.Equal(t, f.projectID.String(), defect.ProjectID)
	assert.Equal(t, f.storyID.String(), defect.Story.ID)
	assert.Equal(t, "Reporte de ventas mensual", defect.Story.Title)
	assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID)
	assert.Equal(t, "active", defect.DetectedSprint.Status)
	assert.Nil(t, defect.ResolvedSprint)
	assert.Equal(t, "null", string(defect.Raw["resolvedSprint"]), "resolvedSprint debe serializarse null")

	// Autoría, estado y marca temporal del estado salen del titular de la sesión y de la creación.
	for name, ref := range map[string]defectUserRef{
		"createdBy": defect.CreatedBy, "updatedBy": defect.UpdatedBy, "statusChangedBy": defect.StatusChangedBy,
	} {
		assert.Equal(t, f.ownerID.String(), ref.ID, name)
		assert.Equal(t, "Ana Pérez", ref.FullName, name)
	}
	assert.NotEmpty(t, defect.CreatedAt)
	assert.Equal(t, defect.CreatedAt, defect.StatusChangedAt, "statusChangedAt de un defecto nuevo es el de su creación")

	// En la base queda una fila Abierta sin sprint de resolución y la historia queda marcada.
	stored := f.env.storedDefect(t, defect.ID)
	assert.Equal(t, "open", string(stored.Status))
	assert.Nil(t, stored.ResolvedSprintID)
	assert.Equal(t, int64(1), f.env.defectCount(t))
	assert.True(t, f.env.everHadDefects(t, f.storyID), "la historia queda marcada como no eliminable")
}

// Escenario: Dado un proyecto con un sprint Activo, cuando un integrante abre el formulario de registro, entonces el sprint Activo viene propuesto por defecto como sprint de detección, y el integrante puede cambiarlo por uno Cerrado (007/US1)
func TestUS1_SprintActivoPorDefecto(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")

	// La propuesta es del frontend; el backend acepta el Activo y también el Cerrado elegido.
	active := requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))
	closed := requireDefectCreated(t, f.post(t, f.body(f.storyID, closedID)))

	assert.Equal(t, "active", active.DetectedSprint.Status)
	assert.Equal(t, "closed", closed.DetectedSprint.Status)
	assert.Equal(t, closedID.String(), closed.DetectedSprint.ID)
	assert.Equal(t, int64(2), f.env.defectCount(t))
}

// Escenario: Dado una historia en estado Completada, cuando un integrante registra un defecto sobre ella, entonces el registro se acepta, porque un defecto suele aparecer justamente después de dar la historia por terminada (007/US1)
func TestUS1_HistoriaCompletada(t *testing.T) {
	f := newDefectFixture(t)
	completedID := f.env.createStory(t, f.ownerToken, f.projectID, "Historia terminada")
	f.env.completeStory(t, f.ownerToken, f.sprintID, completedID)

	defect := requireDefectCreated(t, f.post(t, f.body(completedID, f.sprintID)))

	assert.Equal(t, "open", defect.Status)
	assert.Equal(t, completedID.String(), defect.Story.ID)
	assert.True(t, f.env.everHadDefects(t, completedID))
}

// Escenario: Dado un proyecto sin ningún sprint Activo pero con dos sprints Cerrados, cuando un integrante registra un defecto eligiendo uno de los Cerrados como sprint de detección, entonces el registro se acepta y no se propone ningún sprint por defecto (007/US1)
func TestUS1_SinSprintActivo(t *testing.T) {
	env := newDefectTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProjectInProgress(t, token, "Portal de Reportes")
	storyID := env.createStory(t, token, projectID, "Reporte de ventas mensual")
	env.closedSprint(t, token, projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	secondID := env.closedSprint(t, token, projectID, "Sprint 2", "2026-10-12", "2026-10-16")

	rec := env.postDefect(t, token, projectID, map[string]any{
		"description": defectDescription, "severity": "high",
		"storyId": storyID.String(), "detectedSprintId": secondID.String(),
	})

	defect := requireDefectCreated(t, rec)
	assert.Equal(t, "closed", defect.DetectedSprint.Status)
	assert.Equal(t, secondID.String(), defect.DetectedSprint.ID)
	assert.Equal(t, "open", defect.Status)
}

// Escenario: Dado una historia que ya tiene defectos registrados, cuando un integrante registra otro, entonces se acepta como defecto independiente, sin límite por historia (007/US1)
func TestUS1_VariosDefectosPorHistoria(t *testing.T) {
	f := newDefectFixture(t)

	first := requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))
	second := requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))

	assert.NotEqual(t, first.ID, second.ID, "cada defecto es independiente")
	assert.Equal(t, int64(2), f.env.storyDefectCount(t, f.storyID))
}

// Escenario: Dado un integrante, cuando registra un defecto con una descripción de exactamente 2000 caracteres, entonces el defecto se crea; y cuando tiene 2001, se rechaza (007/US1)
func TestUS1_DescripcionDeDosMilCaracteres(t *testing.T) {
	t.Run("2000 caracteres se acepta", func(t *testing.T) {
		f := newDefectFixture(t)
		body := f.body(f.storyID, f.sprintID)
		body["description"] = strings.Repeat("á", 2000)

		defect := requireDefectCreated(t, f.post(t, body))

		assert.Equal(t, strings.Repeat("á", 2000), defect.Description)
		assert.Equal(t, int64(1), f.env.defectCount(t))
	})

	t.Run("2001 caracteres se rechaza", func(t *testing.T) {
		f := newDefectFixture(t)
		before := f.env.defectCount(t)
		body := f.body(f.storyID, f.sprintID)
		body["description"] = strings.Repeat("á", 2001)

		requireDefectFieldRejected(t, f.post(t, body), "description", defectMsgDescTooLong)

		f.requireNothingCreated(t, before)
	})
}

// Escenario: Dado un integrante, cuando registra un defecto con cada una de las cuatro severidades (Crítica, Alta, Media y Baja), entonces las cuatro se aceptan (007/US1)
func TestUS1_CuatroSeveridadesRegistro(t *testing.T) {
	f := newDefectFixture(t)
	for _, severity := range []string{"critical", "high", "medium", "low"} {
		t.Run(severity, func(t *testing.T) {
			body := f.body(f.storyID, f.sprintID)
			body["severity"] = severity

			defect := requireDefectCreated(t, f.post(t, body))

			assert.Equal(t, severity, defect.Severity)
			assert.Equal(t, severity, string(f.env.storedDefect(t, defect.ID).Severity))
		})
	}
	assert.Equal(t, int64(4), f.env.defectCount(t))
}

// Escenario: Dado una historia con 50 defectos registrados, cuando un integrante registra el número 51, entonces se acepta y la consulta de la historia los devuelve todos (007/US1)
func TestUS1_HistoriaConCincuentaDefectos(t *testing.T) {
	f := newDefectFixture(t)
	for i := 0; i < 50; i++ {
		requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))
	}
	require.Equal(t, int64(50), f.env.storyDefectCount(t, f.storyID), "precondición: la historia tiene 50 defectos")

	requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))

	// La consulta de la historia es de US3; aquí se verifica en la base que quedan los 51.
	assert.Equal(t, int64(51), f.env.storyDefectCount(t, f.storyID))
}

// Escenario: Dado un integrante, cuando intenta registrar un defecto sin descripción o con una descripción compuesta solo por espacios, entonces el registro se rechaza indicando que la descripción es obligatoria (007/US1)
func TestUS1_DescripcionVacia(t *testing.T) {
	t.Run("descripción ausente", func(t *testing.T) {
		f := newDefectFixture(t)
		before := f.env.defectCount(t)
		body := f.body(f.storyID, f.sprintID)
		delete(body, "description")

		requireDefectFieldRejected(t, f.post(t, body), "description", defectMsgDescRequired)

		f.requireNothingCreated(t, before)
	})

	t.Run("descripción de solo espacios", func(t *testing.T) {
		f := newDefectFixture(t)
		before := f.env.defectCount(t)
		body := f.body(f.storyID, f.sprintID)
		body["description"] = "     "

		requireDefectFieldRejected(t, f.post(t, body), "description", defectMsgDescRequired)

		f.requireNothingCreated(t, before)
	})
}

// Escenario: Dado un integrante, cuando intenta registrar un defecto con una severidad que no es ninguna de las cuatro admitidas, entonces el registro se rechaza indicando las severidades válidas (007/US1)
func TestUS1_SeveridadInvalidaRegistro(t *testing.T) {
	f := newDefectFixture(t)
	before := f.env.defectCount(t)
	body := f.body(f.storyID, f.sprintID)
	body["severity"] = "urgent"

	requireDefectFieldRejected(t, f.post(t, body), "severity", defectMsgSeverityInvalid)

	f.requireNothingCreated(t, before)
}

// Escenario: Dado un integrante, cuando intenta registrar un defecto sin historia relacionada o sin sprint de detección, entonces el registro se rechaza indicando el campo faltante (007/US1)
func TestUS1_CamposObligatorios(t *testing.T) {
	t.Run("sin historia relacionada", func(t *testing.T) {
		f := newDefectFixture(t)
		before := f.env.defectCount(t)
		body := f.body(f.storyID, f.sprintID)
		delete(body, "storyId")

		requireDefectFieldRejected(t, f.post(t, body), "storyId", defectMsgStoryRequired)

		f.requireNothingCreated(t, before)
	})

	t.Run("sin sprint de detección", func(t *testing.T) {
		f := newDefectFixture(t)
		before := f.env.defectCount(t)
		body := f.body(f.storyID, f.sprintID)
		delete(body, "detectedSprintId")

		requireDefectFieldRejected(t, f.post(t, body), "detectedSprintId", defectMsgSprintRequired)

		f.requireNothingCreated(t, before)
	})
}

// Escenario: Dado un sprint en estado Planificado, cuando un integrante intenta usarlo como sprint de detección, entonces el registro se rechaza indicando que un defecto solo se detecta en un sprint Activo o Cerrado (007/US1)
func TestUS1_SprintPlanificado(t *testing.T) {
	f := newDefectFixture(t)
	plannedID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 3", "2026-11-02", "2026-11-13")
	before := f.env.defectCount(t)

	requireDefectFieldRejected(t, f.post(t, f.body(f.storyID, plannedID)), "detectedSprintId", defectMsgSprintPlanned)

	f.requireNothingCreated(t, before)
}

// Escenario: Dado una historia o un sprint que pertenece a otro proyecto, cuando un integrante intenta vincularlo al defecto, entonces el registro se rechaza con una respuesta de historia o sprint inexistente, sin revelar dato alguno del otro proyecto (007/US1)
func TestUS1_VinculoDeOtroProyecto(t *testing.T) {
	t.Run("historia de otro proyecto", func(t *testing.T) {
		f := newDefectFixture(t)
		otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
		otherStory := f.env.createStory(t, f.ownerToken, otherProject, "Historia ajena")
		before := f.env.defectCount(t)

		unknown := f.post(t, f.body(uuid.New(), f.sprintID))
		rec := f.post(t, f.body(otherStory, f.sprintID))

		requireNotFoundLikeUnknown(t, rec, unknown, defectMsgStoryNotFound)
		f.requireNothingCreated(t, before)
		assert.False(t, f.env.everHadDefects(t, otherStory), "la historia ajena no se marca")
	})

	t.Run("sprint de otro proyecto", func(t *testing.T) {
		f := newDefectFixture(t)
		otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
		otherSprint := f.env.activeSprint(t, f.ownerToken, otherProject, "Sprint ajeno")
		before := f.env.defectCount(t)

		unknown := f.post(t, f.body(f.storyID, uuid.New()))
		rec := f.post(t, f.body(f.storyID, otherSprint))

		requireNotFoundLikeUnknown(t, rec, unknown, defectMsgSprintNotFound)
		f.requireNothingCreated(t, before)
	})
}

// Escenario: Dado un proyecto en estado Finalizado, cuando un integrante intenta registrar un defecto, entonces el registro se rechaza porque el proyecto es de solo lectura (007/US1)
func TestUS1_ProyectoFinalizado(t *testing.T) {
	f := newDefectFixture(t)
	closeID := f.sprintID
	defectRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/sprints/"+closeID.String()+"/close", f.ownerToken, nil),
		http.StatusOK, nil)
	f.env.setProjectStatus(t, f.ownerToken, f.projectID, "finished")
	before := f.env.defectCount(t)

	body := requireDefectConflict(t, f.post(t, f.body(f.storyID, f.sprintID)), "read_only")

	assert.Equal(t, defectMsgReadOnly, body.Error)
	assert.Equal(t, "null", string(body.CurrentStatus), "currentStatus es null con read_only")
	assert.NotNil(t, body.AllowedStatuses, "allowedStatuses es siempre un arreglo")
	assert.Empty(t, body.AllowedStatuses)
	f.requireNothingCreated(t, before)
}

// Escenario: Dado un usuario que no es integrante del proyecto, cuando intenta registrar un defecto en él, entonces la respuesta es de proyecto inexistente y no se crea nada (007/US1)
func TestUS1_NoIntegrante(t *testing.T) {
	f := newDefectFixture(t)
	_, outsiderToken := f.env.signUp(t, "Carlos Gómez", "carlos@example.com")
	projectNotFound := f.env.do(t, http.MethodGet, "/api/v1/projects/"+f.projectID.String(), outsiderToken, nil)
	require.Equal(t, http.StatusNotFound, projectNotFound.Code, "precondición: la 002 responde 404 a quien no es integrante")
	before := f.env.defectCount(t)

	rec := f.env.postDefect(t, outsiderToken, f.projectID, f.body(f.storyID, f.sprintID))

	// La respuesta es el mismo 404 de proyecto inexistente de la 002.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, projectNotFound.Body.String(), rec.Body.String(), "el cuerpo debe ser el 404 de la 002")
	f.requireNothingCreated(t, before)
}

// Escenario: Dado un proyecto que nunca inició ningún sprint, cuando un integrante intenta registrar un defecto, entonces el registro se rechaza indicando que no hay ningún sprint Activo ni Cerrado disponible como sprint de detección (007/US1)
func TestUS1_ProyectoSinSprintIniciado(t *testing.T) {
	// Cada subtest arma su propio proyecto con una historia y un único sprint Planificado.
	newPlannedOnly := func(t *testing.T) (defectFixture, uuid.UUID) {
		env := newDefectTestEnv(t)
		ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
		projectID := env.createProjectInProgress(t, token, "Portal de Reportes")
		storyID := env.createStory(t, token, projectID, "Reporte de ventas mensual")
		plannedID := env.createSprint(t, token, projectID, "Sprint 1", "2026-11-02", "2026-11-13")
		return defectFixture{env: env, ownerID: ownerID, ownerToken: token, projectID: projectID, storyID: storyID, sprintID: plannedID}, plannedID
	}

	t.Run("con un sprint planificado en el cuerpo", func(t *testing.T) {
		f, plannedID := newPlannedOnly(t)
		before := f.env.defectCount(t)

		body := requireDefectConflict(t, f.post(t, f.body(f.storyID, plannedID)), "no_detection_sprint")

		assert.Equal(t, defectMsgNoDetection, body.Error)
		assert.Equal(t, "null", string(body.CurrentStatus), "currentStatus es null con no_detection_sprint")
		f.requireNothingCreated(t, before)
	})

	t.Run("con un UUID inexistente en el cuerpo", func(t *testing.T) {
		f, _ := newPlannedOnly(t)
		before := f.env.defectCount(t)

		body := requireDefectConflict(t, f.post(t, f.body(f.storyID, uuid.New())), "no_detection_sprint")

		assert.Equal(t, defectMsgNoDetection, body.Error)
		f.requireNothingCreated(t, before)
	})
}

// Verificación de punta a punta del puerto de defectos de la 003 (T022, RC-05): con los endpoints
// reales se registra un defecto sobre una historia y se comprueba que eliminarla se rechaza con 409
// y el motivo defects. No lleva etiqueta 007/USn porque no es un escenario de la spec.
func TestDefectPorts_BloqueanEliminacion(t *testing.T) {
	f := newDefectFixture(t)
	requireDefectCreated(t, f.post(t, f.body(f.storyID, f.sprintID)))

	rec := f.env.do(t, http.MethodDelete, "/api/v1/stories/"+f.storyID.String(), f.ownerToken, nil)

	// El cuerpo es el DeletionBlockedResponse de la 003 con el código defects entre los motivos.
	var blocked struct {
		Error   string `json:"error"`
		Reasons []struct {
			Code string `json:"code"`
		} `json:"reasons"`
	}
	defectRequireStatus(t, rec, http.StatusConflict, &blocked)
	codes := make([]string, 0, len(blocked.Reasons))
	for _, reason := range blocked.Reasons {
		codes = append(codes, reason.Code)
	}
	assert.Contains(t, codes, "defects", "el 409 debe incluir el motivo defects")
}

// Pruebas de integración de punta a punta del seguimiento y la resolución de un defecto (T028, US2):
// atraviesan el router compuesto como en cmd/app, el middleware de sesión de la 001,
// RequireDefectAccess, el servicio y el repositorio de defectos contra smye_test. Cubren
// PUT /api/v1/defects/{defectId}/status y GET /api/v1/defects/{defectId} con un caso por escenario de
// spec.md; los defectos se crean con POST y las transiciones previas con el mismo PUT. En cada
// rechazo se verifica que la fila del defecto quedó idéntica a la previa.

// Textos exactos que fijan el contrato, data-model.md sección 8 y tasks.md (T028) para los rechazos
// del cambio de estado.
const (
	defectMsgResolutionRequired = "El sprint de resolución es obligatorio para resolver."
	defectMsgResolutionBefore   = "El sprint de resolución no puede empezar antes que el de detección."
	defectMsgResolutionPlanned  = "Un defecto solo se resuelve en un sprint Activo o Cerrado."
	defectMsgSkipToResolved     = "Un defecto Abierto primero debe pasar a En progreso."
)

// defectPath es la ruta del detalle de un defecto según contracts/openapi.yaml.
func defectPath(defectID string) string {
	return "/api/v1/defects/" + defectID
}

// putDefectStatus envía PUT /api/v1/defects/{defectId}/status con el estado destino y, si no es
// uuid.Nil, el sprint de resolución.
func (e *defectTestEnv) putDefectStatus(t *testing.T, token, defectID, status string, resolvedSprintID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]any{"status": status}
	if resolvedSprintID != uuid.Nil {
		body["resolvedSprintId"] = resolvedSprintID.String()
	}
	return e.do(t, http.MethodPut, defectPath(defectID)+"/status", token, body)
}

// requireDefectOK exige un 200 cuyo cuerpo sea un defecto y lo devuelve decodificado.
func requireDefectOK(t *testing.T, rec *httptest.ResponseRecorder) defectResponse {
	t.Helper()
	var defect defectResponse
	defectRequireStatus(t, rec, http.StatusOK, &defect)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &defect.Raw), "el cuerpo del 200 no es un objeto JSON")
	return defect
}

// getDefect consulta con GET /api/v1/defects/{defectId} y exige el 200 con el defecto.
func (e *defectTestEnv) getDefect(t *testing.T, token, defectID string) defectResponse {
	t.Helper()
	return requireDefectOK(t, e.do(t, http.MethodGet, defectPath(defectID), token, nil))
}

// registerDefect registra con POST un defecto de la historia detectado en el sprint indicado y
// devuelve su identificador.
func (f defectFixture) registerDefect(t *testing.T, storyID, detectedSprintID uuid.UUID) string {
	t.Helper()
	return requireDefectCreated(t, f.post(t, f.body(storyID, detectedSprintID))).ID
}

// startDefect pasa el defecto a En progreso con el token indicado y exige el 200.
func (f defectFixture) startDefect(t *testing.T, token, defectID string) defectResponse {
	t.Helper()
	defect := requireDefectOK(t, f.env.putDefectStatus(t, token, defectID, "in_progress", uuid.Nil))
	require.Equal(t, "in_progress", defect.Status, "precondición: el defecto quedó En progreso")
	return defect
}

// requireDefectTime exige que el instante serializado coincida con el esperado.
func requireDefectTime(t *testing.T, expected time.Time, raw, name string) {
	t.Helper()
	got, err := time.Parse(time.RFC3339Nano, raw)
	require.NoError(t, err, "%s = %q no es un instante RFC 3339", name, raw)
	assert.True(t, expected.Equal(got), "%s = %s, se esperaba %s", name, got, expected)
}

// requireResolvedIn verifica en la respuesta y en la base que el defecto quedó Resuelto con el
// sprint de resolución indicado y sin cambio en el sprint de detección.
func (f defectFixture) requireResolvedIn(t *testing.T, defect defectResponse, detectedID, resolvedID uuid.UUID) {
	t.Helper()
	assert.Equal(t, "resolved", defect.Status)
	assert.Equal(t, detectedID.String(), defect.DetectedSprint.ID, "el sprint de detección no cambia (FR-027)")
	if assert.NotNil(t, defect.ResolvedSprint, "un defecto Resuelto lleva su sprint de resolución") {
		assert.Equal(t, resolvedID.String(), defect.ResolvedSprint.ID)
	}
	stored := f.env.storedDefect(t, defect.ID)
	assert.Equal(t, "resolved", string(stored.Status))
	assert.Equal(t, detectedID, stored.DetectedSprintID)
	if assert.NotNil(t, stored.ResolvedSprintID, "la fila Resuelta guarda el sprint de resolución") {
		assert.Equal(t, resolvedID, *stored.ResolvedSprintID)
	}
}

// requireDefectRowUnchanged verifica que la fila del defecto en la base es idéntica a la previa.
func (f defectFixture) requireDefectRowUnchanged(t *testing.T, before domain.Defect) {
	t.Helper()
	assert.Equal(t, before, f.env.storedDefect(t, before.ID.String()), "un rechazo no debe modificar el defecto")
}

// Escenario: Dado un defecto Abierto, cuando un integrante lo pasa a En progreso y más tarde a Resuelto indicando el sprint Activo como sprint de resolución, entonces ambas transiciones se aceptan y el defecto queda Resuelto con ese sprint registrado (007/US2)
func TestUS2_CicloCompleto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)

	// Cada transición ocurre en un instante distinto del reloj falso, para identificar la última.
	startedAt := time.Date(2026, time.October, 22, 9, 30, 0, 0, f.env.location)
	f.env.clock.Set(startedAt)
	started := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "in_progress", uuid.Nil))
	assert.Equal(t, "in_progress", started.Status)
	assert.Nil(t, started.ResolvedSprint, "En progreso no tiene sprint de resolución")
	requireDefectTime(t, startedAt, started.StatusChangedAt, "statusChangedAt tras pasar a En progreso")

	resolvedAt := time.Date(2026, time.October, 23, 16, 45, 0, 0, f.env.location)
	f.env.clock.Set(resolvedAt)
	resolved := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", f.sprintID))
	f.requireResolvedIn(t, resolved, f.sprintID, f.sprintID)

	// El detalle muestra el estado, el sprint de resolución y la autoría de la última transición.
	detail := f.env.getDefect(t, f.ownerToken, defectID)
	assert.Equal(t, "resolved", detail.Status)
	if assert.NotNil(t, detail.ResolvedSprint) {
		assert.Equal(t, f.sprintID.String(), detail.ResolvedSprint.ID)
		assert.Equal(t, "Sprint 2", detail.ResolvedSprint.Name)
		assert.Equal(t, "active", detail.ResolvedSprint.Status)
	}
	assert.Equal(t, f.ownerID.String(), detail.StatusChangedBy.ID)
	assert.Equal(t, "Ana Pérez", detail.StatusChangedBy.FullName)
	requireDefectTime(t, resolvedAt, detail.StatusChangedAt, "statusChangedAt del detalle")
	stored := f.env.storedDefect(t, defectID)
	assert.True(t, resolvedAt.Equal(stored.StatusChangedAt), "status_changed_at es el de la última transición")
	assert.Equal(t, f.ownerID, stored.StatusChangedByID)
}

// Escenario: Dado un defecto detectado en el sprint Activo, cuando se resuelve en ese mismo sprint, entonces la resolución se acepta y el defecto queda con el mismo sprint en ambos campos, porque se detectó y se arregló dentro del mismo período (007/US2)
func TestUS2_MismoSprint(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)

	resolved := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", f.sprintID))

	f.requireResolvedIn(t, resolved, f.sprintID, f.sprintID)
	if assert.NotNil(t, resolved.ResolvedSprint) {
		assert.Equal(t, resolved.DetectedSprint.ID, resolved.ResolvedSprint.ID, "el mismo sprint en ambos campos")
	}
}

// Escenario: Dado un defecto detectado en un sprint ya Cerrado, cuando se resuelve en el sprint Activo posterior, entonces la resolución se acepta y los dos sprints quedan registrados por separado, dejando visible que el arreglo llevó más de un sprint (007/US2)
func TestUS2_SprintPosterior(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	defectID := f.registerDefect(t, f.storyID, closedID)
	f.startDefect(t, f.ownerToken, defectID)

	resolved := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", f.sprintID))

	f.requireResolvedIn(t, resolved, closedID, f.sprintID)
	assert.Equal(t, "closed", resolved.DetectedSprint.Status)
	if assert.NotNil(t, resolved.ResolvedSprint) {
		assert.Equal(t, "active", resolved.ResolvedSprint.Status)
		assert.NotEqual(t, resolved.DetectedSprint.ID, resolved.ResolvedSprint.ID, "los dos sprints quedan por separado")
	}
}

// Escenario: Dado un defecto, cuando un integrante distinto del que lo registró lo pasa a En progreso o lo resuelve, entonces la acción se acepta, porque cualquier integrante puede actualizar cualquier defecto del proyecto (007/US2)
func TestUS2_OtroIntegrante(t *testing.T) {
	// newWithMember arma el "Dado" con Beto Ruiz como segundo integrante del proyecto de Ana.
	newWithMember := func(t *testing.T) (defectFixture, uuid.UUID, string) {
		f := newDefectFixture(t)
		memberID, memberToken := f.env.signUp(t, "Beto Ruiz", "beto@example.com")
		f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")
		return f, memberID, memberToken
	}

	// requireActedBy verifica que updatedBy y statusChangedBy son quien actuó y createdBy sigue siendo Ana.
	requireActedBy := func(t *testing.T, f defectFixture, defect defectResponse, actorID uuid.UUID) {
		t.Helper()
		assert.Equal(t, actorID.String(), defect.UpdatedBy.ID, "updatedBy es quien actuó")
		assert.Equal(t, "Beto Ruiz", defect.UpdatedBy.FullName)
		assert.Equal(t, actorID.String(), defect.StatusChangedBy.ID, "statusChangedBy es quien actuó")
		assert.Equal(t, f.ownerID.String(), defect.CreatedBy.ID, "createdBy no cambia")
		assert.Equal(t, "Ana Pérez", defect.CreatedBy.FullName)
		stored := f.env.storedDefect(t, defect.ID)
		assert.Equal(t, actorID, stored.UpdatedByID)
		assert.Equal(t, actorID, stored.StatusChangedByID)
		assert.Equal(t, f.ownerID, stored.CreatedByID)
	}

	t.Run("otro integrante lo pasa a En progreso", func(t *testing.T) {
		f, memberID, memberToken := newWithMember(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)

		started := requireDefectOK(t, f.env.putDefectStatus(t, memberToken, defectID, "in_progress", uuid.Nil))

		assert.Equal(t, "in_progress", started.Status)
		requireActedBy(t, f, started, memberID)
	})

	t.Run("otro integrante lo resuelve", func(t *testing.T) {
		f, memberID, memberToken := newWithMember(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		f.startDefect(t, f.ownerToken, defectID)

		resolved := requireDefectOK(t, f.env.putDefectStatus(t, memberToken, defectID, "resolved", f.sprintID))

		f.requireResolvedIn(t, resolved, f.sprintID, f.sprintID)
		requireActedBy(t, f, resolved, memberID)
	})
}

// Escenario: Dado un defecto detectado en un sprint Cerrado, cuando se lo resuelve indicando otro sprint Cerrado posterior, entonces la resolución se acepta, porque el sprint de resolución puede ser Cerrado y no necesita ser el actual (007/US2)
func TestUS2_ResolucionEnSprintCerrado(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	firstID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	defectID := f.registerDefect(t, f.storyID, firstID)
	f.startDefect(t, f.ownerToken, defectID)
	secondID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 2", "2026-10-12", "2026-10-16")

	resolved := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", secondID))

	f.requireResolvedIn(t, resolved, firstID, secondID)
	if assert.NotNil(t, resolved.ResolvedSprint) {
		assert.Equal(t, "closed", resolved.ResolvedSprint.Status)
	}
}

// Escenario: Dado un defecto cuyo sprint de detección es el único sprint del proyecto, cuando se lo resuelve en ese mismo sprint, entonces la resolución se acepta (007/US2)
func TestUS2_UnicoSprint(t *testing.T) {
	f := newDefectFixture(t)
	var sprints int64
	require.NoError(t, f.env.db.Table("sprints").Where("project_id = ?", f.projectID).Count(&sprints).Error)
	require.Equal(t, int64(1), sprints, "precondición: el proyecto tiene un único sprint")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)

	resolved := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", f.sprintID))

	f.requireResolvedIn(t, resolved, f.sprintID, f.sprintID)
}

// Escenario: Dado un defecto En progreso, cuando un integrante intenta resolverlo sin indicar sprint de resolución, entonces la acción se rechaza indicando que el sprint de resolución es obligatorio para resolver, y el defecto sigue En progreso (007/US2)
func TestUS2_ResolucionSinSprint(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	rec := f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", uuid.Nil)

	requireDefectFieldRejected(t, rec, "resolvedSprintId", defectMsgResolutionRequired)
	f.requireDefectRowUnchanged(t, before)
	assert.Equal(t, "in_progress", f.env.getDefect(t, f.ownerToken, defectID).Status, "el defecto sigue En progreso (SC-006)")
}

// Escenario: Dado un defecto detectado en el segundo sprint del proyecto, cuando un integrante intenta resolverlo indicando el primer sprint, entonces la acción se rechaza indicando que el sprint de resolución no puede empezar antes que el de detección (007/US2)
func TestUS2_ResolucionAnteriorALaDeteccion(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	firstID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	rec := f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", firstID)

	requireDefectFieldRejected(t, rec, "resolvedSprintId", defectMsgResolutionBefore)
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un sprint Planificado, cuando un integrante intenta usarlo como sprint de resolución, entonces la acción se rechaza indicando que solo se resuelve en un sprint Activo o Cerrado (007/US2)
func TestUS2_ResolucionEnSprintPlanificado(t *testing.T) {
	f := newDefectFixture(t)
	plannedID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 3", "2026-11-02", "2026-11-13")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	rec := f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", plannedID)

	requireDefectFieldRejected(t, rec, "resolvedSprintId", defectMsgResolutionPlanned)
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un defecto Abierto, cuando un integrante intenta pasarlo directamente a Resuelto, entonces la acción se rechaza indicando que primero debe pasar por En progreso (007/US2)
func TestUS2_SaltoAResuelto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", f.sprintID), "invalid_transition")

	assert.Equal(t, defectMsgSkipToResolved, body.Error)
	assert.Equal(t, `"open"`, string(body.CurrentStatus), "currentStatus es el estado vigente")
	assert.Contains(t, body.AllowedStatuses, "in_progress", "desde Abierto se permite En progreso")
	assert.NotContains(t, body.AllowedStatuses, "resolved")
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un defecto En progreso, cuando un integrante intenta devolverlo a Abierto, entonces la acción se rechaza indicando las transiciones permitidas; solo se reabre un defecto Resuelto (007/US2)
func TestUS2_VueltaAAbierto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "open", uuid.Nil), "invalid_transition")

	assert.Equal(t, defectMsgOnlyResolvedReopens, body.Error)
	assert.Equal(t, `"in_progress"`, string(body.CurrentStatus), "currentStatus es el estado vigente")
	assert.Contains(t, body.AllowedStatuses, "resolved", "desde En progreso se permite Resuelto")
	assert.NotContains(t, body.AllowedStatuses, "open")
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un sprint de resolución de otro proyecto, cuando un integrante intenta usarlo, entonces la acción se rechaza con una respuesta de sprint inexistente (007/US2)
func TestUS2_SprintDeOtroProyecto(t *testing.T) {
	f := newDefectFixture(t)
	otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
	otherSprint := f.env.activeSprint(t, f.ownerToken, otherProject, "Sprint ajeno")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	unknown := f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", uuid.New())
	rec := f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", otherSprint)

	requireNotFoundLikeUnknown(t, rec, unknown, defectMsgSprintNotFound)
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un proyecto en estado Finalizado, cuando un integrante intenta cambiar el estado de un defecto, entonces la acción se rechaza porque el proyecto es de solo lectura (007/US2)
func TestUS2_ProyectoFinalizado(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	defectRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/sprints/"+f.sprintID.String()+"/close", f.ownerToken, nil),
		http.StatusOK, nil)
	f.env.setProjectStatus(t, f.ownerToken, f.projectID, "finished")
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "in_progress", uuid.Nil), "read_only")

	assert.Equal(t, defectMsgReadOnly, body.Error)
	assert.Equal(t, "null", string(body.CurrentStatus), "currentStatus es null con read_only")
	assert.NotNil(t, body.AllowedStatuses, "allowedStatuses es siempre un arreglo")
	assert.Empty(t, body.AllowedStatuses)
	f.requireDefectRowUnchanged(t, before)
}

// Pruebas de integración de la consulta y el filtrado de defectos (T039): GET
// /api/v1/projects/{projectId}/defects y GET /api/v1/stories/{storyId}/defects sobre el router de
// testenv_test.go. Los "Dado" se preparan con los endpoints reales; solo el estado Descartado, que
// produce otra historia, se fuerza con forceDefectStatus. TestUS3_ValorDeFiltroInvalidoHTTP lleva el
// sufijo "HTTP" para no chocar con la unitaria homónima de validation_test.go.

// Textos del contrato para los rechazos del listado.
const (
	defectMsgFilterForeign = "El filtro no corresponde al proyecto."
	defectMsgStatusInvalid = "El estado debe ser Abierto, En progreso, Resuelto o Descartado."
)

// defectListResponse refleja ProjectDefects y StoryDefects del contrato; Raw distingue `[]` de null.
type defectListResponse struct {
	Defects     []defectResponse           `json:"defects"`
	EmptyReason *string                    `json:"emptyReason"`
	Raw         map[string]json.RawMessage `json:"-"`
}

// requireDefectList exige un 200 cuyo cuerpo sea un listado de defectos y lo devuelve decodificado.
func requireDefectList(t *testing.T, rec *httptest.ResponseRecorder) defectListResponse {
	t.Helper()
	var list defectListResponse
	defectRequireStatus(t, rec, http.StatusOK, &list)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list.Raw), "el cuerpo del 200 no es un objeto JSON")
	return list
}

// defectIDs devuelve los identificadores del listado en el orden recibido.
func defectIDs(list defectListResponse) []string {
	ids := make([]string, 0, len(list.Defects))
	for _, d := range list.Defects {
		ids = append(ids, d.ID)
	}
	return ids
}

// requireEmptyList exige defects: [] (nunca null) y el emptyReason indicado.
func requireEmptyList(t *testing.T, list defectListResponse, reason string) {
	t.Helper()
	assert.Equal(t, "[]", string(list.Raw["defects"]), "defects debe serializarse como []")
	if assert.NotNil(t, list.EmptyReason, "emptyReason no debe ser null con la lista vacía") {
		assert.Equal(t, reason, *list.EmptyReason)
	}
}

// requireNoEmptyReason exige que emptyReason sea null cuando la lista tiene elementos.
func requireNoEmptyReason(t *testing.T, list defectListResponse) {
	t.Helper()
	assert.Equal(t, "null", string(list.Raw["emptyReason"]), "emptyReason es null si hay defectos")
}

// listProject consulta el listado del proyecto con la query indicada (con el "?" inicial o vacía).
func (f defectFixture) listProject(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	return f.env.do(t, http.MethodGet, defectsPath(f.projectID)+query, f.ownerToken, nil)
}

// listStory consulta el listado de la historia.
func (f defectFixture) listStory(t *testing.T, storyID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return f.env.do(t, http.MethodGet, "/api/v1/stories/"+storyID.String()+"/defects", f.ownerToken, nil)
}

// registerWithSeverity registra un defecto con la severidad indicada y devuelve su identificador.
func (f defectFixture) registerWithSeverity(t *testing.T, severity string, storyID, detectedSprintID uuid.UUID) string {
	t.Helper()
	body := f.body(storyID, detectedSprintID)
	body["severity"] = severity
	return requireDefectCreated(t, f.post(t, body)).ID
}

// resolveDefect lleva el defecto a En progreso y de ahí a Resuelto en el sprint indicado.
func (f defectFixture) resolveDefect(t *testing.T, defectID string, resolvedSprintID uuid.UUID) {
	t.Helper()
	f.startDefect(t, f.ownerToken, defectID)
	requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", resolvedSprintID))
}

// defectRank replica el orden por severidad del listado: crítica, alta, media y baja.
func defectRank(severity domain.DefectSeverity) int {
	switch string(severity) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

// listingOrder ordena los identificadores como debe hacerlo el listado (FR-044): rango de severidad,
// created_at descendente e id descendente, leyendo esos valores de la base.
func (e *defectTestEnv) listingOrder(t *testing.T, ids []string) []string {
	t.Helper()
	stored := make(map[string]domain.Defect, len(ids))
	for _, id := range ids {
		stored[id] = e.storedDefect(t, id)
	}
	ordered := append([]string{}, ids...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := stored[ordered[i]], stored[ordered[j]]
		if ra, rb := defectRank(a.Severity), defectRank(b.Severity); ra != rb {
			return ra < rb
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		return ordered[i] > ordered[j]
	})
	return ordered
}

// requireListing exige que el listado contenga exactamente los defectos esperados y en el orden
// estable del contrato.
func (f defectFixture) requireListing(t *testing.T, list defectListResponse, want ...string) {
	t.Helper()
	assert.Equal(t, f.env.listingOrder(t, want), defectIDs(list))
}

// us3Seed es el "Dado" común de US3: dos historias, un sprint Cerrado y uno Activo, y seis defectos
// de las cuatro severidades y los cuatro estados.
type us3Seed struct {
	defectFixture
	closedID uuid.UUID
	storyA   uuid.UUID
	storyB   uuid.UUID
	// Defectos por nombre: d1 crítico Abierto (A, detectado en el Cerrado); d2 alto En progreso (A,
	// Cerrado); d3 medio Resuelto (B, Cerrado, resuelto en el Activo); d4 bajo Descartado (B, Activo);
	// d5 crítico En progreso (B, Activo); d6 alto Resuelto (A, detectado y resuelto en el Activo).
	d map[string]string
}

// newUS3Seed arma el "Dado" común de US3 con los endpoints reales.
func newUS3Seed(t *testing.T) us3Seed {
	t.Helper()
	f := newDefectFixtureWithoutSprint(t)
	closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	s := us3Seed{defectFixture: f, closedID: closedID, storyA: f.storyID, d: map[string]string{}}
	s.storyB = f.env.createStory(t, f.ownerToken, f.projectID, "Exportación a PDF")

	s.d["d1"] = f.registerWithSeverity(t, "critical", s.storyA, closedID)
	s.d["d2"] = f.registerWithSeverity(t, "high", s.storyA, closedID)
	f.startDefect(t, f.ownerToken, s.d["d2"])
	s.d["d3"] = f.registerWithSeverity(t, "medium", s.storyB, closedID)
	f.resolveDefect(t, s.d["d3"], f.sprintID)
	s.d["d4"] = f.registerWithSeverity(t, "low", s.storyB, f.sprintID)
	f.env.forceDefectStatus(t, defectParseID(t, s.d["d4"]), domain.DefectStatus("discarded"), nil)
	s.d["d5"] = f.registerWithSeverity(t, "critical", s.storyB, f.sprintID)
	f.startDefect(t, f.ownerToken, s.d["d5"])
	s.d["d6"] = f.registerWithSeverity(t, "high", s.storyA, f.sprintID)
	f.resolveDefect(t, s.d["d6"], f.sprintID)
	return s
}

// want devuelve los identificadores de los defectos con los nombres indicados.
func (s us3Seed) want(names ...string) []string {
	ids := make([]string, 0, len(names))
	for _, name := range names {
		ids = append(ids, s.d[name])
	}
	return ids
}

// Escenario: Dado un proyecto con defectos en los cuatro estados y de las cuatro severidades, cuando un integrante abre la lista sin filtros, entonces ve todos los defectos con su descripción, severidad, estado, historia, sprint de detección y sprint de resolución cuando lo tienen (007/US3)
func TestUS3_SinFiltros(t *testing.T) {
	s := newUS3Seed(t)

	list := requireDefectList(t, s.listProject(t, ""))

	s.requireListing(t, list, s.want("d1", "d2", "d3", "d4", "d5", "d6")...)
	requireNoEmptyReason(t, list)
	type expected struct {
		severity, status, story, detected, resolved string
	}
	cases := map[string]expected{
		"d1": {"critical", "open", "Reporte de ventas mensual", "Sprint 1", ""},
		"d2": {"high", "in_progress", "Reporte de ventas mensual", "Sprint 1", ""},
		"d3": {"medium", "resolved", "Exportación a PDF", "Sprint 1", "Sprint 2"},
		"d4": {"low", "discarded", "Exportación a PDF", "Sprint 2", ""},
		"d5": {"critical", "in_progress", "Exportación a PDF", "Sprint 2", ""},
		"d6": {"high", "resolved", "Reporte de ventas mensual", "Sprint 2", "Sprint 2"},
	}
	byID := map[string]defectResponse{}
	for _, d := range list.Defects {
		byID[d.ID] = d
	}
	for name, want := range cases {
		d := byID[s.d[name]]
		assert.Equal(t, defectDescription, d.Description, name)
		assert.Equal(t, want.severity, d.Severity, name)
		assert.Equal(t, want.status, d.Status, name)
		assert.Equal(t, want.story, d.Story.Title, name)
		assert.Equal(t, want.detected, d.DetectedSprint.Name, name)
		if want.resolved == "" {
			assert.Nil(t, d.ResolvedSprint, name)
		} else if assert.NotNil(t, d.ResolvedSprint, name) {
			assert.Equal(t, want.resolved, d.ResolvedSprint.Name, name)
		}
	}
}

// Escenario: Dado una historia con defectos de distintos estados, cuando un integrante consulta los defectos de esa historia, entonces ve solo los de ella, de cualquier estado, con los mismos datos y el mismo orden que en la lista del proyecto (007/US3)
func TestUS3_DefectosDeUnaHistoria(t *testing.T) {
	s := newUS3Seed(t)

	listA := requireDefectList(t, s.listStory(t, s.storyA))
	listB := requireDefectList(t, s.listStory(t, s.storyB))

	s.requireListing(t, listA, s.want("d1", "d2", "d6")...)
	s.requireListing(t, listB, s.want("d3", "d4", "d5")...)
	requireNoEmptyReason(t, listA)

	// Los datos de cada elemento coinciden con los del listado del proyecto.
	project := requireDefectList(t, s.listProject(t, "?storyId="+s.storyB.String()))
	assert.Equal(t, project.Defects, listB.Defects)
}

// Escenario: Dado un proyecto con defectos de distintos estados, cuando un integrante filtra por uno o varios estados, entonces la lista contiene solo los defectos de esos estados (007/US3)
func TestUS3_FiltroPorEstado(t *testing.T) {
	s := newUS3Seed(t)

	t.Run("un estado", func(t *testing.T) {
		list := requireDefectList(t, s.listProject(t, "?status=resolved"))
		s.requireListing(t, list, s.want("d3", "d6")...)
	})

	t.Run("varios estados excluyen los descartados", func(t *testing.T) {
		list := requireDefectList(t, s.listProject(t, "?status=open&status=in_progress&status=resolved"))
		s.requireListing(t, list, s.want("d1", "d2", "d3", "d5", "d6")...)
	})

	t.Run("descartados", func(t *testing.T) {
		list := requireDefectList(t, s.listProject(t, "?status=discarded"))
		s.requireListing(t, list, s.want("d4")...)
	})
}

// Escenario: Dado un proyecto con defectos de distintas severidades y estados, cuando un integrante filtra por severidad Crítica y estado En progreso, entonces la lista contiene solo los que cumplen ambos filtros (007/US3)
func TestUS3_SeveridadYEstado(t *testing.T) {
	s := newUS3Seed(t)

	list := requireDefectList(t, s.listProject(t, "?severity=critical&status=in_progress"))

	s.requireListing(t, list, s.want("d5")...)
}

// Escenario: Dado un sprint con defectos detectados en él y defectos resueltos en él, cuando un integrante filtra por ese sprint, entonces ve ambos grupos y un defecto detectado y resuelto en el mismo sprint aparece una sola vez (007/US3)
func TestUS3_FiltroPorSprint(t *testing.T) {
	s := newUS3Seed(t)

	list := requireDefectList(t, s.listProject(t, "?sprintId="+s.sprintID.String()))

	// d3 solo se resolvió en el sprint; d4 y d5 solo se detectaron; d6 se detectó y se resolvió en él.
	s.requireListing(t, list, s.want("d3", "d4", "d5", "d6")...)
	count := 0
	for _, d := range list.Defects {
		if d.ID == s.d["d6"] {
			count++
		}
	}
	assert.Equal(t, 1, count, "el defecto detectado y resuelto en el sprint aparece una sola vez")
}

// Escenario: Dado un proyecto sin ningún defecto, cuando un integrante abre la lista, entonces recibe una lista vacía que indica que el proyecto no tiene defectos (007/US3)
func TestUS3_ProyectoSinDefectos(t *testing.T) {
	f := newDefectFixture(t)

	list := requireDefectList(t, f.listProject(t, ""))

	requireEmptyList(t, list, "no_defects")
}

// Escenario: Dado una historia sin defectos, cuando un integrante consulta sus defectos, entonces recibe una lista vacía y no un error (007/US3)
func TestUS3_HistoriaSinDefectos(t *testing.T) {
	s := newUS3Seed(t)
	emptyStory := s.env.createStory(t, s.ownerToken, s.projectID, "Historia sin defectos")

	list := requireDefectList(t, s.listStory(t, emptyStory))

	requireEmptyList(t, list, "no_defects")
}

// Escenario: Dado un proyecto con defectos, cuando un integrante aplica filtros que ningún defecto cumple, entonces recibe una lista vacía que indica que no hay coincidencias, distinta de la del proyecto sin defectos (007/US3)
func TestUS3_SinCoincidencias(t *testing.T) {
	f := newDefectFixture(t)
	f.registerDefect(t, f.storyID, f.sprintID)

	list := requireDefectList(t, f.listProject(t, "?status=resolved"))

	requireEmptyList(t, list, "no_matches")
}

// Escenario: Dado una historia con 50 defectos de severidades mezcladas, cuando un integrante los consulta varias veces, entonces recibe los 50 siempre en el mismo orden: severidad, más recientes primero y desempate por id (007/US3)
func TestUS3_OrdenEstable(t *testing.T) {
	f := newDefectFixture(t)
	severities := []string{"low", "critical", "medium", "high"}
	ids := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		ids = append(ids, f.registerWithSeverity(t, severities[(i*3+i/4)%4], f.storyID, f.sprintID))
	}
	expected := f.env.listingOrder(t, ids)

	for attempt := 1; attempt <= 3; attempt++ {
		list := requireDefectList(t, f.listStory(t, f.storyID))
		assert.Equal(t, expected, defectIDs(list), "consulta %d de la historia", attempt)
	}
	assert.Equal(t, expected, defectIDs(requireDefectList(t, f.listProject(t, ""))), "el listado del proyecto usa el mismo orden")
}

// Escenario: Dado una historia o un sprint que pertenece a otro proyecto, cuando un integrante lo usa como filtro, entonces la respuesta es un error de filtro idéntico al de un identificador inexistente, sin revelar nada del otro proyecto (007/US3)
func TestUS3_FiltroDeOtroProyecto(t *testing.T) {
	cases := []struct {
		name  string
		field string
		// foreign arma el identificador ajeno en otro proyecto del mismo titular.
		foreign func(t *testing.T, f defectFixture, otherProject uuid.UUID) uuid.UUID
	}{
		{"historia ajena", "storyId", func(t *testing.T, f defectFixture, other uuid.UUID) uuid.UUID {
			return f.env.createStory(t, f.ownerToken, other, "Historia ajena")
		}},
		{"sprint ajeno", "sprintId", func(t *testing.T, f defectFixture, other uuid.UUID) uuid.UUID {
			return f.env.activeSprint(t, f.ownerToken, other, "Sprint ajeno")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDefectFixture(t)
			otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
			foreign := tc.foreign(t, f, otherProject)

			unknown := f.listProject(t, "?"+tc.field+"="+uuid.New().String())
			rec := f.listProject(t, "?"+tc.field+"="+foreign.String())

			requireDefectFieldRejected(t, unknown, tc.field, defectMsgFilterForeign)
			requireDefectFieldRejected(t, rec, tc.field, defectMsgFilterForeign)
			assert.Equal(t, unknown.Body.String(), rec.Body.String(), "el 400 debe ser idéntico al de un UUID inexistente")
		})
	}
}

// Escenario: Dado un filtro con un estado o una severidad que no existen, cuando un integrante consulta la lista, entonces la respuesta es un error de entrada inválida que informa el campo (007/US3)
func TestUS3_ValorDeFiltroInvalidoHTTP(t *testing.T) {
	f := newDefectFixture(t)
	f.registerDefect(t, f.storyID, f.sprintID)

	t.Run("estado", func(t *testing.T) {
		requireDefectFieldRejected(t, f.listProject(t, "?status=closed"), "status", defectMsgStatusInvalid)
	})

	t.Run("severidad", func(t *testing.T) {
		requireDefectFieldRejected(t, f.listProject(t, "?severity=urgent"), "severity", defectMsgSeverityInvalid)
	})
}

// Escenario: Dado un usuario que no es integrante del proyecto, cuando intenta consultar los defectos del proyecto o de una historia, entonces recibe el mismo 404 de proyecto o de historia inexistente de las specs 002 y 003 (007/US3)
func TestUS3_NoIntegrante(t *testing.T) {
	f := newDefectFixture(t)
	f.registerDefect(t, f.storyID, f.sprintID)
	_, outsiderToken := f.env.signUp(t, "Carlos Gómez", "carlos@example.com")

	t.Run("listado del proyecto", func(t *testing.T) {
		projectNotFound := f.env.do(t, http.MethodGet, "/api/v1/projects/"+f.projectID.String(), outsiderToken, nil)
		require.Equal(t, http.StatusNotFound, projectNotFound.Code, "precondición: la 002 responde 404")

		rec := f.env.do(t, http.MethodGet, defectsPath(f.projectID), outsiderToken, nil)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, projectNotFound.Body.String(), rec.Body.String(), "el cuerpo debe ser el 404 de la 002")
	})

	t.Run("listado de la historia", func(t *testing.T) {
		storyNotFound := f.env.do(t, http.MethodGet, "/api/v1/stories/"+f.storyID.String(), outsiderToken, nil)
		require.Equal(t, http.StatusNotFound, storyNotFound.Code, "precondición: la 003 responde 404")

		rec := f.env.do(t, http.MethodGet, "/api/v1/stories/"+f.storyID.String()+"/defects", outsiderToken, nil)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, storyNotFound.Body.String(), rec.Body.String(), "el cuerpo debe ser el 404 de la 003")
	})
}

// Escenario: Dado un proyecto con defectos, cuando un integrante combina dos o más filtros, entonces la lista contiene exactamente el subconjunto que cumple todos (007/US3)
func TestUS3_CombinacionesDeFiltros(t *testing.T) {
	s := newUS3Seed(t)
	sprint, storyA, storyB := s.sprintID.String(), s.storyA.String(), s.storyB.String()
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"severidad alta y estado resuelto", "?severity=high&status=resolved", []string{"d6"}},
		{"historia B y severidad crítica", "?storyId=" + storyB + "&severity=critical", []string{"d5"}},
		{"sprint y estado resuelto", "?sprintId=" + sprint + "&status=resolved", []string{"d3", "d6"}},
		{"historia A y sprint", "?storyId=" + storyA + "&sprintId=" + sprint, []string{"d6"}},
		{"dos severidades y historia A", "?severity=critical&severity=high&storyId=" + storyA, []string{"d1", "d2", "d6"}},
		{"cuatro filtros", "?status=in_progress&severity=critical&storyId=" + storyB + "&sprintId=" + sprint, []string{"d5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := requireDefectList(t, s.listProject(t, tc.query))
			s.requireListing(t, list, s.want(tc.want...)...)
		})
	}
}

// Pruebas de integración de la corrección de los datos de un defecto (T048, US4): atraviesan el
// router compuesto como en cmd/app, el middleware de sesión de la 001, RequireDefectAccess, el
// servicio y el repositorio de defectos contra smye_test. Cubren PATCH /api/v1/defects/{defectId} con
// un caso por escenario de spec.md; los "Dado" se preparan con los endpoints reales y solo los
// estados que producen otras historias (Descartado, reabierto) se fuerzan con forceDefectStatus. En
// cada rechazo se verifica que la fila del defecto quedó idéntica a la previa (FR-037).

// Textos exactos que fijan el contrato y data-model.md para los rechazos de la modificación.
const (
	defectMsgStatusNotEditable = "El estado y el sprint de resolución se cambian con las acciones de estado del defecto."
	defectMsgDiscarded         = "El defecto está Descartado y es de solo lectura."
	defectMsgLinksFrozen       = "La historia relacionada y el sprint de detección solo se corrigen mientras el defecto está Abierto."
)

// patchDefect envía PATCH /api/v1/defects/{defectId} con el cuerpo recibido.
func (e *defectTestEnv) patchDefect(t *testing.T, token, defectID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPatch, defectPath(defectID), token, body)
}

// requireLinksFrozen exige el 409 links_frozen con el estado vigente indicado y el defecto intacto.
func (f defectFixture) requireLinksFrozen(t *testing.T, rec *httptest.ResponseRecorder, currentStatus string, before domain.Defect) {
	t.Helper()
	body := requireDefectConflict(t, rec, "links_frozen")
	assert.Equal(t, defectMsgLinksFrozen, body.Error)
	assert.Equal(t, `"`+currentStatus+`"`, string(body.CurrentStatus))
	f.requireDefectRowUnchanged(t, before)
}

// requireOnlyEditableChanged verifica que entre las dos filas solo difieren los campos que una
// modificación puede tocar: el estado, el sprint de resolución, el autor y los datos del último
// cambio de estado nunca cambian (FR-035).
func requireOnlyEditableChanged(t *testing.T, before, after domain.Defect) {
	t.Helper()
	neutral := func(d domain.Defect) domain.Defect {
		d.Description, d.Severity, d.StoryID, d.DetectedSprintID = "", "", uuid.Nil, uuid.Nil
		d.UpdatedAt, d.UpdatedByID = time.Time{}, uuid.Nil
		return d
	}
	assert.Equal(t, neutral(before), neutral(after), "una modificación no toca estado, sprint de resolución ni autor")
}

// Escenario: Dado un defecto Abierto con severidad Media, cuando un integrante corrige su descripción y le sube la severidad a Crítica, entonces el defecto queda actualizado y su estado, sus sprints y su autor no cambian (007/US4)
func TestUS4_DescripcionYSeveridad(t *testing.T) {
	f := newDefectFixture(t)
	memberID, memberToken := f.env.signUp(t, "Beto Gómez", "beto@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")
	defectID := f.registerWithSeverity(t, "medium", f.storyID, f.sprintID)
	before := f.env.storedDefect(t, defectID)

	defect := requireDefectOK(t, f.env.patchDefect(t, memberToken, defectID, map[string]any{
		"description": "El total del reporte no suma los descuentos por volumen",
		"severity":    "critical",
	}))

	assert.Equal(t, "El total del reporte no suma los descuentos por volumen", defect.Description)
	assert.Equal(t, "critical", defect.Severity)
	assert.Equal(t, "open", defect.Status)
	assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID)
	assert.Nil(t, defect.ResolvedSprint)
	assert.Equal(t, f.storyID.String(), defect.Story.ID)
	assert.Equal(t, f.ownerID.String(), defect.CreatedBy.ID, "el autor no cambia")
	assert.Equal(t, memberID.String(), defect.UpdatedBy.ID, "updatedBy es quien modificó")
	stored := f.env.storedDefect(t, defectID)
	assert.Equal(t, memberID, stored.UpdatedByID)
	assert.Equal(t, "critical", string(stored.Severity))
	requireOnlyEditableChanged(t, before, stored)
	detail := f.env.getDefect(t, f.ownerToken, defectID)
	assert.Equal(t, defect.Description, detail.Description, "el detalle refleja la modificación")
	assert.Equal(t, defect.Severity, detail.Severity)
}

// Escenario: Dado un defecto Abierto vinculado a la historia equivocada, cuando un integrante lo revincula a otra historia del mismo proyecto, entonces el cambio se acepta y el defecto deja de aparecer en la historia anterior (007/US4)
func TestUS4_RevinculacionDeHistoria(t *testing.T) {
	f := newDefectFixture(t)
	otherStory := f.env.createStory(t, f.ownerToken, f.projectID, "Exportación a PDF")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	require.False(t, f.env.everHadDefects(t, otherStory), "precondición: la historia nueva nunca tuvo defectos")

	defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"storyId": otherStory.String()}))

	assert.Equal(t, otherStory.String(), defect.Story.ID)
	assert.Empty(t, requireDefectList(t, f.listStory(t, f.storyID)).Defects, "deja de aparecer en la historia anterior")
	assert.Equal(t, []string{defectID}, defectIDs(requireDefectList(t, f.listStory(t, otherStory))))
	assert.True(t, f.env.everHadDefects(t, f.storyID), "la historia anterior conserva ever_had_defects")
	assert.True(t, f.env.everHadDefects(t, otherStory), "la historia nueva queda marcada con ever_had_defects")
}

// Escenario: Dado un defecto ya Resuelto, cuando un integrante corrige su descripción, entonces el cambio se acepta, porque corregir el texto no altera el resultado del seguimiento (007/US4)
func TestUS4_DescripcionDeResuelto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.resolveDefect(t, defectID, f.sprintID)
	before := f.env.storedDefect(t, defectID)

	defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"description": "Descripción corregida"}))

	assert.Equal(t, "Descripción corregida", defect.Description)
	f.requireResolvedIn(t, defect, f.sprintID, f.sprintID)
	requireOnlyEditableChanged(t, before, f.env.storedDefect(t, defectID))
}

// Escenario: Dado un defecto En progreso, cuando un integrante le cambia la severidad, entonces el cambio se acepta y el defecto sigue En progreso (007/US4)
func TestUS4_SeveridadEnProgreso(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"severity": "low"}))

	assert.Equal(t, "low", defect.Severity)
	assert.Equal(t, "in_progress", defect.Status)
	stored := f.env.storedDefect(t, defectID)
	assert.Equal(t, "low", string(stored.Severity))
	requireOnlyEditableChanged(t, before, stored)
}

// Escenario: Dado un defecto Abierto registrado con el sprint de detección equivocado, cuando un integrante lo corrige por otro sprint Activo o Cerrado del proyecto, entonces el cambio se acepta, porque el defecto todavía no empezó a trabajarse (007/US4)
func TestUS4_CorreccionDeSprintDeDeteccion(t *testing.T) {
	t.Run("otro sprint Activo", func(t *testing.T) {
		f := newDefectFixtureWithoutSprint(t)
		closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
		f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
		defectID := f.registerDefect(t, f.storyID, closedID)
		before := f.env.storedDefect(t, defectID)

		defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"detectedSprintId": f.sprintID.String()}))

		assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID)
		stored := f.env.storedDefect(t, defectID)
		assert.Equal(t, f.sprintID, stored.DetectedSprintID)
		requireOnlyEditableChanged(t, before, stored)
	})

	t.Run("un sprint Cerrado", func(t *testing.T) {
		f := newDefectFixtureWithoutSprint(t)
		closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
		f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		before := f.env.storedDefect(t, defectID)

		defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"detectedSprintId": closedID.String()}))

		assert.Equal(t, closedID.String(), defect.DetectedSprint.ID)
		stored := f.env.storedDefect(t, defectID)
		assert.Equal(t, closedID, stored.DetectedSprintID)
		requireOnlyEditableChanged(t, before, stored)
	})
}

// Escenario: Dado un defecto Abierto, cuando un integrante le corrige la historia relacionada y el sprint de detección en la misma operación, entonces ambos cambios se aceptan, porque los dos vínculos comparten la misma ventana de corrección (007/US4)
func TestUS4_AmbosVinculos(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	otherStory := f.env.createStory(t, f.ownerToken, f.projectID, "Exportación a PDF")
	defectID := f.registerDefect(t, f.storyID, closedID)

	defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{
		"storyId":          otherStory.String(),
		"detectedSprintId": f.sprintID.String(),
	}))

	assert.Equal(t, otherStory.String(), defect.Story.ID)
	assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID)
	stored := f.env.storedDefect(t, defectID)
	assert.Equal(t, otherStory, stored.StoryID)
	assert.Equal(t, f.sprintID, stored.DetectedSprintID)
}

// Escenario: Dado un defecto, cuando un integrante lo deja con exactamente la misma descripción y severidad que ya tenía, entonces el cambio se acepta sin efectos (007/US4)
func TestUS4_SinCambios(t *testing.T) {
	t.Run("descripción y severidad iguales en un defecto Abierto", func(t *testing.T) {
		f := newDefectFixture(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		before := f.env.storedDefect(t, defectID)

		defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"description": defectDescription, "severity": "high"}))

		assert.Equal(t, defectDescription, defect.Description)
		assert.Equal(t, "high", defect.Severity)
		stored := f.env.storedDefect(t, defectID)
		requireOnlyEditableChanged(t, before, stored)
		assert.Equal(t, before.Description, stored.Description)
		assert.Equal(t, before.Severity, stored.Severity)
	})

	t.Run("los dos vínculos iguales en un defecto En progreso", func(t *testing.T) {
		f := newDefectFixture(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		f.startDefect(t, f.ownerToken, defectID)
		before := f.env.storedDefect(t, defectID)

		defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{
			"storyId":          f.storyID.String(),
			"detectedSprintId": f.sprintID.String(),
		}))

		assert.Equal(t, "in_progress", defect.Status)
		stored := f.env.storedDefect(t, defectID)
		requireOnlyEditableChanged(t, before, stored)
		assert.Equal(t, before.StoryID, stored.StoryID)
		assert.Equal(t, before.DetectedSprintID, stored.DetectedSprintID)
	})
}

// Escenario: Dado un defecto, cuando un integrante lleva su descripción a exactamente 2000 caracteres, entonces el cambio se acepta (007/US4)
func TestUS4_DescripcionDeDosMilCaracteres(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	text := strings.Repeat("ñ", 2000)

	defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"description": text}))

	assert.Equal(t, text, defect.Description)
	assert.Equal(t, text, f.env.storedDefect(t, defectID).Description)
}

// Escenario: Dado un defecto que fue resuelto y después reabierto, cuando un integrante le corrige el sprint de detección, entonces el cambio se acepta, porque el defecto volvió a estar Abierto y la ventana de corrección se abre de nuevo (007/US4)
func TestUS4_VentanaReabierta(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.resolveDefect(t, defectID, f.sprintID)
	// La reapertura (US5) deja el defecto open sin sprint de resolución; se prepara directamente.
	f.env.forceDefectStatus(t, defectParseID(t, defectID), domain.DefectStatus("open"), nil)

	defect := requireDefectOK(t, f.env.patchDefect(t, f.ownerToken, defectID,
		map[string]any{"detectedSprintId": closedID.String()}))

	assert.Equal(t, "open", defect.Status)
	assert.Equal(t, closedID.String(), defect.DetectedSprint.ID)
	assert.Equal(t, closedID, f.env.storedDefect(t, defectID).DetectedSprintID)
}

// Escenario: Dado un defecto, cuando un integrante intenta dejar la descripción vacía o poner una severidad inválida, entonces el cambio se rechaza con el motivo y el defecto queda exactamente como estaba (007/US4)
func TestUS4_DatosInvalidos(t *testing.T) {
	t.Run("descripción vacía", func(t *testing.T) {
		f := newDefectFixture(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		before := f.env.storedDefect(t, defectID)

		requireDefectFieldRejected(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"description": "   "}), "description", defectMsgDescRequired)

		f.requireDefectRowUnchanged(t, before)
	})

	t.Run("severidad inválida", func(t *testing.T) {
		f := newDefectFixture(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		before := f.env.storedDefect(t, defectID)

		requireDefectFieldRejected(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"description": "Texto válido", "severity": "urgent"}), "severity", defectMsgSeverityInvalid)

		f.requireDefectRowUnchanged(t, before)
	})
}

// Escenario: Dado un defecto Abierto, cuando un integrante intenta revincularlo a una historia de otro proyecto, entonces el cambio se rechaza con una respuesta de historia inexistente, aunque esté dentro de la ventana de corrección (007/US4)
func TestUS4_HistoriaDeOtroProyecto(t *testing.T) {
	f := newDefectFixture(t)
	otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
	otherStory := f.env.createStory(t, f.ownerToken, otherProject, "Historia ajena")
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	before := f.env.storedDefect(t, defectID)

	unknown := f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"storyId": uuid.NewString()})
	rec := f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"storyId": otherStory.String()})

	requireNotFoundLikeUnknown(t, rec, unknown, defectMsgStoryNotFound)
	f.requireDefectRowUnchanged(t, before)
	assert.False(t, f.env.everHadDefects(t, otherStory), "la historia ajena no se marca")
}

// Escenario: Dado un defecto En progreso o Resuelto, cuando un integrante intenta corregirle el sprint de detección o la historia relacionada, entonces el cambio se rechaza indicando que los vínculos del defecto solo se corrigen mientras está Abierto (007/US4)
func TestUS4_VinculosCongelados(t *testing.T) {
	for _, status := range []string{"in_progress", "resolved"} {
		for _, link := range []string{"historia", "sprint de detección"} {
			t.Run(link+" con el defecto "+status, func(t *testing.T) {
				f := newDefectFixtureWithoutSprint(t)
				closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
				f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
				otherStory := f.env.createStory(t, f.ownerToken, f.projectID, "Exportación a PDF")
				defectID := f.registerDefect(t, f.storyID, f.sprintID)
				if status == "in_progress" {
					f.startDefect(t, f.ownerToken, defectID)
				} else {
					f.resolveDefect(t, defectID, f.sprintID)
				}
				before := f.env.storedDefect(t, defectID)
				body := map[string]any{"storyId": otherStory.String()}
				if link != "historia" {
					body = map[string]any{"detectedSprintId": closedID.String()}
				}

				f.requireLinksFrozen(t, f.env.patchDefect(t, f.ownerToken, defectID, body), status, before)

				assert.False(t, f.env.everHadDefects(t, otherStory), "un rechazo no marca la historia")
			})
		}
	}
}

// Escenario: Dado un defecto Abierto, cuando un integrante intenta corregir su sprint de detección por uno Planificado o por uno de otro proyecto, entonces el cambio se rechaza con el mismo criterio que en el registro (007/US4)
func TestUS4_SprintDeDeteccionInvalido(t *testing.T) {
	t.Run("sprint Planificado", func(t *testing.T) {
		f := newDefectFixture(t)
		plannedID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 3", "2026-11-02", "2026-11-13")
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		before := f.env.storedDefect(t, defectID)

		requireDefectFieldRejected(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"detectedSprintId": plannedID.String()}), "detectedSprintId", defectMsgSprintPlanned)

		f.requireDefectRowUnchanged(t, before)
	})

	t.Run("sprint de otro proyecto", func(t *testing.T) {
		f := newDefectFixture(t)
		otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
		otherSprint := f.env.activeSprint(t, f.ownerToken, otherProject, "Sprint ajeno")
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		before := f.env.storedDefect(t, defectID)

		unknown := f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"detectedSprintId": uuid.NewString()})
		rec := f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"detectedSprintId": otherSprint.String()})

		requireNotFoundLikeUnknown(t, rec, unknown, defectMsgSprintNotFound)
		f.requireDefectRowUnchanged(t, before)
	})
}

// Escenario: Dado un defecto Descartado, cuando un integrante intenta modificar cualquiera de sus datos, entonces la acción se rechaza porque un defecto descartado es de solo lectura (007/US4)
func TestUS4_DefectoDescartado(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.env.forceDefectStatus(t, defectParseID(t, defectID), domain.DefectStatus("discarded"), nil)
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.patchDefect(t, f.ownerToken, defectID,
		map[string]any{"description": "Descripción corregida"}), "defect_discarded")

	assert.Equal(t, defectMsgDiscarded, body.Error)
	assert.Equal(t, `"discarded"`, string(body.CurrentStatus))
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un defecto, cuando un integrante intenta cambiar su estado o su sprint de resolución desde la modificación de datos, entonces la acción se rechaza indicando que el estado se cambia con las transiciones de US2, US5 y US6 (007/US4)
func TestUS4_EstadoDesdeModificacion(t *testing.T) {
	cases := []struct {
		field string
		value string
	}{
		{"status", "in_progress"},
		{"resolvedSprintId", uuid.NewString()},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			f := newDefectFixture(t)
			defectID := f.registerDefect(t, f.storyID, f.sprintID)
			before := f.env.storedDefect(t, defectID)

			requireDefectFieldRejected(t, f.env.patchDefect(t, f.ownerToken, defectID,
				map[string]any{"severity": "low", tc.field: tc.value}), tc.field, defectMsgStatusNotEditable)

			f.requireDefectRowUnchanged(t, before)
		})
	}
}

// Escenario: Dado un proyecto en estado Finalizado, cuando un integrante intenta modificar un defecto, entonces la acción se rechaza porque el proyecto es de solo lectura (007/US4)
func TestUS4_ProyectoFinalizado(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	defectRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/sprints/"+f.sprintID.String()+"/close", f.ownerToken, nil),
		http.StatusOK, nil)
	f.env.setProjectStatus(t, f.ownerToken, f.projectID, "finished")
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.patchDefect(t, f.ownerToken, defectID,
		map[string]any{"description": "Descripción corregida"}), "read_only")

	assert.Equal(t, defectMsgReadOnly, body.Error)
	assert.Equal(t, "null", string(body.CurrentStatus), "currentStatus es null con read_only")
	f.requireDefectRowUnchanged(t, before)
}

// Pruebas de integración de la reapertura de un defecto Resuelto (T058, US5): PUT
// /api/v1/defects/{defectId}/status con el estado open sobre el router de testenv_test.go.

// Texto del contrato para la reapertura de un defecto que no está Resuelto.
const defectMsgOnlyResolvedReopens = "Solo se reabre un defecto Resuelto."

// reopenDefect reabre el defecto con el token del propietario y exige el 200 con el defecto Abierto.
func (f defectFixture) reopenDefect(t *testing.T, defectID string) defectResponse {
	t.Helper()
	defect := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "open", uuid.Nil))
	require.Equal(t, "open", defect.Status, "la reapertura deja el defecto Abierto")
	return defect
}

// resolveFromOpen pasa un defecto Abierto por En progreso y lo resuelve en el sprint indicado.
func (f defectFixture) resolveFromOpen(t *testing.T, defectID string, resolvedSprintID uuid.UUID) defectResponse {
	t.Helper()
	f.startDefect(t, f.ownerToken, defectID)
	return requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", resolvedSprintID))
}

// Escenario: Dado un defecto Resuelto con su sprint de resolución registrado, cuando un integrante lo reabre, entonces el defecto vuelve a Abierto, su sprint de resolución queda vacío y su sprint de detección no cambia (007/US5)
func TestUS5_Reapertura(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.resolveFromOpen(t, defectID, f.sprintID)
	resolvedRow := f.env.storedDefect(t, defectID)

	defect := f.reopenDefect(t, defectID)

	assert.Equal(t, "null", string(defect.Raw["resolvedSprint"]), "resolvedSprint debe serializarse null (SC-009)")
	assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID, "el sprint de detección no cambia (FR-027)")
	assert.Equal(t, f.storyID.String(), defect.Story.ID, "la historia no cambia")
	stored := f.env.storedDefect(t, defectID)
	assert.Equal(t, "open", string(stored.Status))
	assert.Nil(t, stored.ResolvedSprintID, "la fila no conserva el sprint de resolución")
	assert.Equal(t, resolvedRow.DetectedSprintID, stored.DetectedSprintID)
	assert.Equal(t, resolvedRow.StoryID, stored.StoryID)
	detail := f.env.getDefect(t, f.ownerToken, defectID)
	assert.Equal(t, "open", detail.Status)
	assert.Equal(t, "null", string(detail.Raw["resolvedSprint"]))
}

// Escenario: Dado un defecto reabierto, cuando un integrante lo vuelve a pasar por En progreso y lo resuelve en un sprint posterior, entonces la segunda resolución se acepta y el sprint de resolución queda con el sprint nuevo (007/US5)
func TestUS5_SegundaResolucion(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	firstID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	defectID := f.registerDefect(t, f.storyID, firstID)
	f.resolveFromOpen(t, defectID, firstID)
	f.reopenDefect(t, defectID)

	resolved := f.resolveFromOpen(t, defectID, f.sprintID)

	f.requireResolvedIn(t, resolved, firstID, f.sprintID)
}

// Escenario: Dado un defecto que se reabrió y se resolvió dos veces, cuando un integrante lo consulta, entonces ve su estado actual y el sprint de resolución vigente, que es siempre el de la última resolución (007/US5)
func TestUS5_VariasReaperturas(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	firstID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	secondID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 2", "2026-10-12", "2026-10-16")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 3")
	defectID := f.registerDefect(t, f.storyID, firstID)
	f.resolveFromOpen(t, defectID, secondID)
	f.reopenDefect(t, defectID)
	f.resolveFromOpen(t, defectID, f.sprintID)

	detail := f.env.getDefect(t, f.ownerToken, defectID)

	f.requireResolvedIn(t, detail, firstID, f.sprintID)
	if assert.NotNil(t, detail.ResolvedSprint) {
		assert.NotEqual(t, secondID.String(), detail.ResolvedSprint.ID, "vale la última resolución, no la primera")
	}
}

// Escenario: Dado un defecto detectado en el primer sprint y resuelto en el segundo, cuando se lo reabre y se lo resuelve de nuevo en el segundo sprint, entonces la resolución se acepta, porque la regla compara contra el sprint de detección, que no cambió (007/US5)
func TestUS5_ResolucionEnElMismoSprint(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	firstID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	defectID := f.registerDefect(t, f.storyID, firstID)
	f.resolveFromOpen(t, defectID, f.sprintID)
	f.reopenDefect(t, defectID)

	resolved := f.resolveFromOpen(t, defectID, f.sprintID)

	f.requireResolvedIn(t, resolved, firstID, f.sprintID)
}

// Escenario: Dado un defecto Resuelto en un proyecto cuyo único sprint ya está Cerrado, cuando un integrante lo reabre, entonces la reapertura se acepta y el defecto queda Abierto, aunque no haya ningún sprint Activo (007/US5)
func TestUS5_SinSprintActivo(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	closedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	defectID := f.registerDefect(t, f.storyID, closedID)
	f.resolveFromOpen(t, defectID, closedID)

	defect := f.reopenDefect(t, defectID)

	assert.Equal(t, "open", defect.Status)
	assert.Equal(t, "null", string(defect.Raw["resolvedSprint"]))
	assert.Equal(t, closedID.String(), defect.DetectedSprint.ID)
	assert.Equal(t, "open", string(f.env.storedDefect(t, defectID).Status))
}

// Escenario: Dado un defecto Abierto o En progreso, cuando un integrante intenta reabrirlo, entonces la acción se rechaza indicando que solo se reabre un defecto Resuelto (007/US5)
func TestUS5_DefectoNoResuelto(t *testing.T) {
	cases := []struct {
		name    string
		current string
		prepare func(t *testing.T, f defectFixture, defectID string)
	}{
		{"Abierto", "open", func(t *testing.T, f defectFixture, defectID string) {}},
		{"En progreso", "in_progress", func(t *testing.T, f defectFixture, defectID string) { f.startDefect(t, f.ownerToken, defectID) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDefectFixture(t)
			defectID := f.registerDefect(t, f.storyID, f.sprintID)
			tc.prepare(t, f, defectID)
			before := f.env.storedDefect(t, defectID)

			body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "open", uuid.Nil), "invalid_transition")

			assert.Equal(t, defectMsgOnlyResolvedReopens, body.Error)
			assert.Equal(t, `"`+tc.current+`"`, string(body.CurrentStatus), "currentStatus es el estado vigente")
			f.requireDefectRowUnchanged(t, before)
		})
	}
}

// Escenario: Dado un proyecto en estado Finalizado, cuando un integrante intenta reabrir un defecto, entonces la acción se rechaza porque el proyecto es de solo lectura (007/US5)
func TestUS5_ProyectoFinalizado(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.resolveFromOpen(t, defectID, f.sprintID)
	defectRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/sprints/"+f.sprintID.String()+"/close", f.ownerToken, nil),
		http.StatusOK, nil)
	f.env.setProjectStatus(t, f.ownerToken, f.projectID, "finished")
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "open", uuid.Nil), "read_only")

	assert.Equal(t, defectMsgReadOnly, body.Error)
	assert.Equal(t, "null", string(body.CurrentStatus), "currentStatus es null con read_only")
	f.requireDefectRowUnchanged(t, before)
}

// Pruebas de integración del descarte de un defecto (T063, US6): PUT /api/v1/defects/{defectId}/status
// con el estado discarded sobre el router de testenv_test.go.

// Textos del contrato para el descarte de un Resuelto y para cualquier transición desde un Descartado.
const (
	defectMsgOnlyOpenDiscards = "Solo se descarta un defecto Abierto o En progreso; para descartarlo, primero reabrilo."
	defectMsgDiscardedFinal   = "El defecto está Descartado, que es un estado definitivo."
)

// discardDefect descarta el defecto con el token del propietario y exige el 200 con el defecto Descartado.
func (f defectFixture) discardDefect(t *testing.T, defectID string) defectResponse {
	t.Helper()
	defect := requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "discarded", uuid.Nil))
	require.Equal(t, "discarded", defect.Status, "el descarte deja el defecto Descartado")
	return defect
}

// requireDiscardedStored verifica en la base que el defecto está Descartado, conserva su detección y
// no tiene sprint de resolución.
func (f defectFixture) requireDiscardedStored(t *testing.T, defectID string) {
	t.Helper()
	stored := f.env.storedDefect(t, defectID)
	assert.Equal(t, "discarded", string(stored.Status))
	assert.Equal(t, f.sprintID, stored.DetectedSprintID)
	assert.Nil(t, stored.ResolvedSprintID, "un descartado nunca tiene sprint de resolución")
}

// Escenario: Dado un defecto Abierto que resultó ser un duplicado, cuando un integrante lo descarta, entonces el defecto queda en estado Descartado, conserva su sprint de detección y sigue sin sprint de resolución (007/US6)
func TestUS6_DescarteDesdeAbierto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)

	defect := f.discardDefect(t, defectID)

	assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID, "conserva el sprint de detección")
	assert.Equal(t, "null", string(defect.Raw["resolvedSprint"]), "resolvedSprint debe serializarse null")
	f.requireDiscardedStored(t, defectID)
}

// Escenario: Dado un defecto En progreso que al investigarlo resultó no ser un defecto, cuando un integrante lo descarta, entonces el descarte se acepta, porque también se llega a Descartado desde En progreso (007/US6)
func TestUS6_DescarteDesdeEnProgreso(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)

	defect := f.discardDefect(t, defectID)

	assert.Equal(t, f.sprintID.String(), defect.DetectedSprint.ID)
	assert.Equal(t, "null", string(defect.Raw["resolvedSprint"]))
	f.requireDiscardedStored(t, defectID)
}

// Escenario: Dado un defecto Descartado, cuando un integrante consulta los defectos del proyecto sin filtros, entonces lo ve listado con su estado Descartado, distinguible de los demás (007/US6)
func TestUS6_ListadoConDescartados(t *testing.T) {
	f := newDefectFixture(t)
	keptID := f.registerDefect(t, f.storyID, f.sprintID)
	discardedID := f.registerDefect(t, f.storyID, f.sprintID)
	f.discardDefect(t, discardedID)

	list := requireDefectList(t, f.listProject(t, ""))

	f.requireListing(t, list, keptID, discardedID)
	for _, d := range list.Defects {
		if d.ID == discardedID {
			assert.Equal(t, "discarded", d.Status)
		} else {
			assert.Equal(t, "open", d.Status)
		}
	}
}

// Escenario: Dado un proyecto con defectos en los cuatro estados, cuando un integrante filtra por estado Descartado, entonces ve únicamente los descartados (007/US6)
func TestUS6_FiltroPorDescartado(t *testing.T) {
	s := newUS3Seed(t)
	// Se descarta con el endpoint real un defecto más, además del d4 sembrado.
	s.discardDefect(t, s.d["d1"])

	t.Run("solo descartados", func(t *testing.T) {
		list := requireDefectList(t, s.listProject(t, "?status=discarded"))
		s.requireListing(t, list, s.want("d1", "d4")...)
	})

	t.Run("los otros tres estados lo excluyen", func(t *testing.T) {
		list := requireDefectList(t, s.listProject(t, "?status=open&status=in_progress&status=resolved"))
		s.requireListing(t, list, s.want("d2", "d3", "d5", "d6")...)
	})
}

// Escenario: Dado una historia cuyos únicos defectos fueron todos descartados, cuando un integrante intenta eliminar esa historia, entonces la eliminación se rechaza igual, porque la historia tuvo defectos asociados y eso no se revierte (007/US6)
func TestUS6_HistoriaNoEliminable(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.discardDefect(t, defectID)

	rec := f.env.do(t, http.MethodDelete, "/api/v1/stories/"+f.storyID.String(), f.ownerToken, nil)

	var blocked struct {
		Reasons []struct {
			Code string `json:"code"`
		} `json:"reasons"`
	}
	defectRequireStatus(t, rec, http.StatusConflict, &blocked)
	codes := make([]string, 0, len(blocked.Reasons))
	for _, reason := range blocked.Reasons {
		codes = append(codes, reason.Code)
	}
	assert.Contains(t, codes, "defects", "el 409 debe incluir el motivo defects (RN-19)")
}

// Escenario: Dado un defecto Descartado, cuando un integrante consulta su detalle, entonces ve su sprint de detección y el campo de sprint de resolución vacío, porque el defecto nunca se resolvió (007/US6)
func TestUS6_DetalleDeDescartado(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.discardDefect(t, defectID)

	detail := f.env.getDefect(t, f.ownerToken, defectID)

	assert.Equal(t, "discarded", detail.Status)
	assert.Equal(t, f.sprintID.String(), detail.DetectedSprint.ID)
	assert.Equal(t, "null", string(detail.Raw["resolvedSprint"]))
}

// Escenario: Dado un defecto Resuelto, cuando un integrante intenta descartarlo, entonces la acción se rechaza indicando que solo se descarta un defecto Abierto o En progreso; para sacarlo de circulación hay que reabrirlo primero (007/US6)
func TestUS6_DescarteDeResuelto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.resolveFromOpen(t, defectID, f.sprintID)
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "discarded", uuid.Nil), "invalid_transition")

	assert.Equal(t, defectMsgOnlyOpenDiscards, body.Error)
	assert.Equal(t, `"resolved"`, string(body.CurrentStatus), "currentStatus es el estado vigente")
	f.requireDefectRowUnchanged(t, before)
}

// Escenario: Dado un defecto Descartado, cuando un integrante intenta descartarlo de nuevo, pasarlo a En progreso, resolverlo o reabrirlo, entonces la acción se rechaza porque Descartado es un estado definitivo (007/US6)
func TestUS6_DescartadoDefinitivo(t *testing.T) {
	cases := []struct {
		name   string
		status string
	}{
		{"descartar", "discarded"},
		{"pasar a En progreso", "in_progress"},
		{"resolver", "resolved"},
		{"reabrir", "open"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDefectFixture(t)
			defectID := f.registerDefect(t, f.storyID, f.sprintID)
			f.discardDefect(t, defectID)
			before := f.env.storedDefect(t, defectID)
			resolvedSprint := uuid.Nil
			if tc.status == "resolved" {
				resolvedSprint = f.sprintID
			}

			body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, tc.status, resolvedSprint), "invalid_transition")

			assert.Equal(t, defectMsgDiscardedFinal, body.Error)
			assert.Equal(t, `"discarded"`, string(body.CurrentStatus))
			f.requireDefectRowUnchanged(t, before)
		})
	}

	t.Run("modificar", func(t *testing.T) {
		f := newDefectFixture(t)
		defectID := f.registerDefect(t, f.storyID, f.sprintID)
		f.discardDefect(t, defectID)
		before := f.env.storedDefect(t, defectID)

		body := requireDefectConflict(t, f.env.patchDefect(t, f.ownerToken, defectID,
			map[string]any{"description": "Descripción corregida"}), "defect_discarded")

		assert.Equal(t, defectMsgDiscarded, body.Error)
		assert.Equal(t, `"discarded"`, string(body.CurrentStatus))
		f.requireDefectRowUnchanged(t, before)
	})
}

// Escenario: Dado un proyecto en estado Finalizado, cuando un integrante intenta descartar un defecto, entonces la acción se rechaza porque el proyecto es de solo lectura (007/US6)
func TestUS6_ProyectoFinalizado(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	defectRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/sprints/"+f.sprintID.String()+"/close", f.ownerToken, nil),
		http.StatusOK, nil)
	f.env.setProjectStatus(t, f.ownerToken, f.projectID, "finished")
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "discarded", uuid.Nil), "read_only")

	assert.Equal(t, defectMsgReadOnly, body.Error)
	assert.Equal(t, "null", string(body.CurrentStatus), "currentStatus es null con read_only")
	f.requireDefectRowUnchanged(t, before)
}

// Pruebas de caracterización de la fase Polish (T068): fijan reglas transversales que ya cumple la
// implementación (sesión, 404 indistinguibles, vínculos inmutables, autor removido, sin hash de
// contraseña). No llevan etiqueta 007/USn porque no son escenarios de una historia, y por eso no
// tienen RED: pasan desde el primer momento.

// requireNoPasswordHash exige que el cuerpo no exponga el hash de contraseña (Principio VI).
func requireNoPasswordHash(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.NotContains(t, rec.Body.String(), "password_hash", "la respuesta no debe exponer password_hash")
	assert.NotContains(t, rec.Body.String(), "passwordHash", "la respuesta no debe exponer passwordHash")
}

// tamperedToken devuelve el token con el primer carácter de la firma cambiado. Se cambia el primero
// y no el último porque el último carácter de una firma de 32 bytes en base64url solo aporta 4 bits
// útiles: cambiarlo por otro que difiera solo en los bits de relleno deja la misma firma y el token
// seguiría siendo válido (la prueba fallaba en el 6% de las corridas).
func tamperedToken(token string) string {
	start := strings.LastIndex(token, ".") + 1
	replacement := byte('A')
	if token[start] == 'A' {
		replacement = 'B'
	}
	return token[:start] + string(replacement) + token[start+1:]
}

// TestDefectEndpoints_SinSesion verifica que los seis endpoints rechazan con 401 la petición sin
// cabecera de autorización y con token adulterado, sin modificar nada (SC-016).
func TestDefectEndpoints_SinSesion(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	before := f.env.storedDefect(t, defectID)
	count := f.env.defectCount(t)
	endpoints := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"alta", http.MethodPost, defectsPath(f.projectID), f.body(f.storyID, f.sprintID)},
		{"listado del proyecto", http.MethodGet, defectsPath(f.projectID), nil},
		{"listado de la historia", http.MethodGet, "/api/v1/stories/" + f.storyID.String() + "/defects", nil},
		{"detalle", http.MethodGet, defectPath(defectID), nil},
		{"modificación", http.MethodPatch, defectPath(defectID), map[string]any{"description": "Descripción adulterada"}},
		{"cambio de estado", http.MethodPut, defectPath(defectID) + "/status", map[string]any{"status": "in_progress"}},
	}
	tokens := []struct{ name, value string }{
		{"sin cabecera", ""},
		{"token adulterado", tamperedToken(f.ownerToken)},
	}
	for _, ep := range endpoints {
		for _, tk := range tokens {
			t.Run(ep.name+" "+tk.name, func(t *testing.T) {
				rec := f.env.do(t, ep.method, ep.path, tk.value, ep.body)

				assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
				requireNoPasswordHash(t, rec)
			})
		}
	}
	assert.Equal(t, count, f.env.defectCount(t), "ningún rechazo crea defectos")
	f.requireDefectRowUnchanged(t, before)
}

// TestDefectEndpoints_NotFoundIndistinguible compara byte a byte el 404 (y el 400 de filtro) de un
// recurso ajeno con el de uno inexistente (SC-017).
func TestDefectEndpoints_NotFoundIndistinguible(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	_, outsiderToken := f.env.signUp(t, "Carlos Gómez", "carlos@example.com")
	missing := uuid.NewString()

	// requireSame exige dos respuestas con el mismo código y el mismo cuerpo exacto.
	requireSame := func(t *testing.T, code int, foreign, unknown *httptest.ResponseRecorder) {
		t.Helper()
		assert.Equal(t, code, unknown.Code, "precondición: el inexistente responde %d", code)
		assert.Equal(t, code, foreign.Code, "cuerpo: %s", foreign.Body.String())
		assert.Equal(t, unknown.Body.String(), foreign.Body.String(), "debe ser idéntico byte a byte")
		requireNoPasswordHash(t, foreign)
	}

	t.Run("proyecto", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			var body any
			if method == http.MethodPost {
				body = f.body(f.storyID, f.sprintID)
			}
			foreign := f.env.do(t, method, defectsPath(f.projectID), outsiderToken, body)
			unknown := f.env.do(t, method, "/api/v1/projects/"+missing+"/defects", outsiderToken, body)
			requireSame(t, http.StatusNotFound, foreign, unknown)
		}
	})

	t.Run("historia", func(t *testing.T) {
		foreign := f.env.do(t, http.MethodGet, "/api/v1/stories/"+f.storyID.String()+"/defects", outsiderToken, nil)
		unknown := f.env.do(t, http.MethodGet, "/api/v1/stories/"+missing+"/defects", outsiderToken, nil)
		requireSame(t, http.StatusNotFound, foreign, unknown)
	})

	t.Run("defecto", func(t *testing.T) {
		calls := []struct {
			method, suffix string
			body           any
		}{
			{http.MethodGet, "", nil},
			{http.MethodPatch, "", map[string]any{"description": "Otra descripción"}},
			{http.MethodPut, "/status", map[string]any{"status": "in_progress"}},
		}
		for _, call := range calls {
			foreign := f.env.do(t, call.method, defectPath(defectID)+call.suffix, outsiderToken, call.body)
			unknown := f.env.do(t, call.method, defectPath(missing)+call.suffix, outsiderToken, call.body)
			requireSame(t, http.StatusNotFound, foreign, unknown)
		}
	})

	t.Run("historia y sprint en el cuerpo", func(t *testing.T) {
		otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto")
		otherStory := f.env.createStory(t, f.ownerToken, otherProject, "Historia ajena")
		otherSprint := f.env.activeSprint(t, f.ownerToken, otherProject, "Sprint ajeno")
		inProgressID := f.registerDefect(t, f.storyID, f.sprintID)
		f.startDefect(t, f.ownerToken, inProgressID)
		openID := f.registerDefect(t, f.storyID, f.sprintID)

		// Alta: historia y sprint.
		requireSame(t, http.StatusNotFound,
			f.post(t, f.body(otherStory, f.sprintID)), f.post(t, f.body(uuid.New(), f.sprintID)))
		requireSame(t, http.StatusNotFound,
			f.post(t, f.body(f.storyID, otherSprint)), f.post(t, f.body(f.storyID, uuid.New())))
		// Cambio de estado: sprint de resolución.
		requireSame(t, http.StatusNotFound,
			f.env.putDefectStatus(t, f.ownerToken, inProgressID, "resolved", otherSprint),
			f.env.putDefectStatus(t, f.ownerToken, inProgressID, "resolved", uuid.New()))
		// Modificación: historia y sprint de detección de un defecto Abierto.
		requireSame(t, http.StatusNotFound,
			f.env.patchDefect(t, f.ownerToken, openID, map[string]any{"storyId": otherStory.String()}),
			f.env.patchDefect(t, f.ownerToken, openID, map[string]any{"storyId": uuid.NewString()}))
		requireSame(t, http.StatusNotFound,
			f.env.patchDefect(t, f.ownerToken, openID, map[string]any{"detectedSprintId": otherSprint.String()}),
			f.env.patchDefect(t, f.ownerToken, openID, map[string]any{"detectedSprintId": uuid.NewString()}))
	})

	t.Run("filtro ajeno e inexistente", func(t *testing.T) {
		otherProject := f.env.createProjectInProgress(t, f.ownerToken, "Otro proyecto B")
		foreign := map[string]uuid.UUID{
			"storyId":  f.env.createStory(t, f.ownerToken, otherProject, "Historia ajena B"),
			"sprintId": f.env.activeSprint(t, f.ownerToken, otherProject, "Sprint ajeno B"),
		}
		for field, id := range foreign {
			rec := f.listProject(t, "?"+field+"="+id.String())
			unknown := f.listProject(t, "?"+field+"="+uuid.NewString())
			requireSame(t, http.StatusBadRequest, rec, unknown)
		}
	})
}

// TestDefectLinks_Immutable recorre en progreso, resolución, reapertura, segunda resolución y
// descarte, y verifica que la historia y el sprint de detección no cambian (SC-010).
func TestDefectLinks_Immutable(t *testing.T) {
	f := newDefectFixtureWithoutSprint(t)
	detectedID := f.env.closedSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-05", "2026-10-09")
	f.sprintID = f.env.activeSprint(t, f.ownerToken, f.projectID, "Sprint 2")
	defectID := f.registerDefect(t, f.storyID, detectedID)
	first := f.env.getDefect(t, f.ownerToken, defectID)

	// requireLinks exige que el defecto conserve la historia y el sprint de detección del alta.
	requireLinks := func(step string, defect defectResponse) {
		assert.Equal(t, first.Story.ID, defect.Story.ID, "historia tras %s", step)
		assert.Equal(t, first.DetectedSprint.ID, defect.DetectedSprint.ID, "sprint de detección tras %s", step)
	}

	requireLinks("pasar a En progreso", f.startDefect(t, f.ownerToken, defectID))
	requireLinks("la resolución", requireDefectOK(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "resolved", f.sprintID)))
	requireLinks("la reapertura", f.reopenDefect(t, defectID))
	requireLinks("la segunda resolución", f.resolveFromOpen(t, defectID, f.sprintID))
	f.reopenDefect(t, defectID)
	requireLinks("el descarte", f.discardDefect(t, defectID))

	requireLinks("la consulta final", f.env.getDefect(t, f.ownerToken, defectID))
	stored := f.env.storedDefect(t, defectID)
	assert.Equal(t, f.storyID, stored.StoryID)
	assert.Equal(t, detectedID, stored.DetectedSprintID)
}

// TestDefectAuthor_RemovedMember verifica que los defectos de quien dejó el proyecto siguen en el
// listado y el detalle con su nombre, y que otro integrante los sigue trabajando (FR-055, SC-018).
func TestDefectAuthor_RemovedMember(t *testing.T) {
	f := newDefectFixture(t)
	removedID, removedToken := f.env.signUp(t, "Beto Gómez", "beto@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")
	otherID, otherToken := f.env.signUp(t, "Carla Ruiz", "carla@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "carla@example.com")

	// Beto registra tres defectos.
	registered := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		registered = append(registered, requireDefectCreated(t, f.env.postDefect(t, removedToken, f.projectID, f.body(f.storyID, f.sprintID))).ID)
	}

	// La propietaria lo quita con la 002.
	rec := f.env.do(t, http.MethodDelete, "/api/v1/projects/"+f.projectID.String()+"/members/"+removedID.String(), f.ownerToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "cuerpo: %s", rec.Body.String())

	// El listado conserva todos, con su nombre como autor.
	listRec := f.listProject(t, "")
	requireNoPasswordHash(t, listRec)
	list := requireDefectList(t, listRec)
	assert.ElementsMatch(t, registered, defectIDs(list))
	for _, d := range list.Defects {
		assert.Equal(t, removedID.String(), d.CreatedBy.ID)
		assert.Equal(t, "Beto Gómez", d.CreatedBy.FullName)
	}

	// El detalle también, y otro integrante los trabaja.
	for _, id := range registered {
		detail := f.env.getDefect(t, f.ownerToken, id)
		assert.Equal(t, "Beto Gómez", detail.CreatedBy.FullName)
		assert.Equal(t, removedID.String(), detail.CreatedBy.ID)
	}
	started := requireDefectOK(t, f.env.putDefectStatus(t, otherToken, registered[0], "in_progress", uuid.Nil))
	assert.Equal(t, "in_progress", started.Status)
	assert.Equal(t, otherID.String(), started.StatusChangedBy.ID)
	assert.Equal(t, "Beto Gómez", started.CreatedBy.FullName, "el autor no cambia")
	edited := requireDefectOK(t, f.env.patchDefect(t, otherToken, registered[1], map[string]any{"description": "Corregida por Carla"}))
	assert.Equal(t, "Corregida por Carla", edited.Description)
	assert.Equal(t, "Beto Gómez", edited.CreatedBy.FullName)

	// Quien fue removido ya no accede.
	gone := f.env.do(t, http.MethodGet, defectPath(registered[0]), removedToken, nil)
	assert.Equal(t, http.StatusNotFound, gone.Code)
}

// TestDefectEndpoints_SinPasswordHash verifica que ninguna respuesta exitosa de los seis endpoints
// contiene password_hash ni passwordHash (Principio VI).
func TestDefectEndpoints_SinPasswordHash(t *testing.T) {
	f := newDefectFixture(t)
	created := f.post(t, f.body(f.storyID, f.sprintID))
	defectID := requireDefectCreated(t, created).ID
	responses := []*httptest.ResponseRecorder{
		created,
		f.listProject(t, ""),
		f.listStory(t, f.storyID),
		f.env.do(t, http.MethodGet, defectPath(defectID), f.ownerToken, nil),
		f.env.patchDefect(t, f.ownerToken, defectID, map[string]any{"description": "Otra descripción"}),
		f.env.putDefectStatus(t, f.ownerToken, defectID, "in_progress", uuid.Nil),
	}
	for i, rec := range responses {
		assert.Less(t, rec.Code, 300, "respuesta %d: %s", i, rec.Body.String())
		requireNoPasswordHash(t, rec)
	}
}
