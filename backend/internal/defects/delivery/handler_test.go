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
	defectMsgBackToOpen         = "Para volver a Abierto, resolvé el defecto y después reabrilo."
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

// Escenario: Dado un defecto En progreso, cuando un integrante intenta devolverlo a Abierto, entonces la acción se rechaza indicando las transiciones permitidas; para volver a Abierto hay que resolverlo y después reabrirlo (007/US2)
func TestUS2_VueltaAAbierto(t *testing.T) {
	f := newDefectFixture(t)
	defectID := f.registerDefect(t, f.storyID, f.sprintID)
	f.startDefect(t, f.ownerToken, defectID)
	before := f.env.storedDefect(t, defectID)

	body := requireDefectConflict(t, f.env.putDefectStatus(t, f.ownerToken, defectID, "open", uuid.Nil), "invalid_transition")

	assert.Equal(t, defectMsgBackToOpen, body.Error)
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
