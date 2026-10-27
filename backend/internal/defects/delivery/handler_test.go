package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
