package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// Pruebas de integración de punta a punta del registro de esfuerzo: atraviesan el router de Gin
// compuesto como en cmd/app (testenv_test.go), el middleware de sesión de la 001, el
// RequireStoryAccess de la 003, el servicio y el repositorio de esfuerzo contra la base smye_test
// (Principios II y V). Las de US1 (T015) cubren POST /api/v1/stories/{storyId}/effort-entries con un
// caso por escenario de spec.md; los "Dado" de proyectos, historias y sprints se preparan con los
// endpoints reales de la 002, la 003 y la 004, y solo las horas ya cargadas de otros días o proyectos
// se siembran en la base. En cada rechazo se verifica que effort_entries no cambió.

// Textos exactos que fijan el contrato y data-model.md sección 7 para el 400 de entrada inválida y
// los rechazos de fecha y de solo lectura.
const (
	effortInvalidFieldsError = "Hay campos inválidos en la solicitud."
	effortMsgDateInFuture    = "No se registran horas en una fecha futura."
	effortMsgDateBeforeStart = "La fecha no puede ser anterior al inicio del proyecto (AAAA-MM-DD)."
	effortMsgReadOnly        = "El proyecto está finalizado y su esfuerzo es de solo lectura."
)

// effortActivity es la actividad del caso normal de US1.
const effortActivity = "Implementación del formulario de alta"

// effortUserRef refleja UserRef del contrato.
type effortUserRef struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
}

// effortStoryRef refleja StoryRef del contrato.
type effortStoryRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// effortSprintRef refleja SprintRef del contrato.
type effortSprintRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// effortEntryResponse refleja EffortEntry del contrato. Hours se conserva como texto literal para
// comprobar los 2 decimales, y Raw guarda el cuerpo para distinguir sprint: null de un campo ausente.
type effortEntryResponse struct {
	ID         string                     `json:"id"`
	EntryDate  string                     `json:"entryDate"`
	Activity   string                     `json:"activity"`
	Hours      json.RawMessage            `json:"hours"`
	Author     effortUserRef              `json:"author"`
	Story      effortStoryRef             `json:"story"`
	Sprint     *effortSprintRef           `json:"sprint"`
	Definitive bool                       `json:"definitive"`
	CreatedAt  string                     `json:"createdAt"`
	UpdatedAt  string                     `json:"updatedAt"`
	Raw        map[string]json.RawMessage `json:"-"`
}

// effortConflictResponse refleja EffortConflictResponse del contrato; availableHours se conserva
// como texto literal para comprobar null y los 2 decimales.
type effortConflictResponse struct {
	Code           string          `json:"code"`
	Error          string          `json:"error"`
	AvailableHours json.RawMessage `json:"availableHours"`
}

// effortValidationResponse refleja ValidationErrorResponse del contrato.
type effortValidationResponse struct {
	Error  string                    `json:"error"`
	Fields []authdelivery.FieldError `json:"fields"`
}

// requireEffortCreated exige un 201 cuyo cuerpo sea el registro creado y lo devuelve decodificado.
func requireEffortCreated(t *testing.T, rec *httptest.ResponseRecorder) effortEntryResponse {
	t.Helper()
	var entry effortEntryResponse
	effortRequireStatus(t, rec, http.StatusCreated, &entry)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entry.Raw), "el cuerpo del 201 no es un objeto JSON")
	return entry
}

// requireEffortFieldRejected exige un 400 de entrada inválida que informe el campo con el mensaje
// indicado.
func requireEffortFieldRejected(t *testing.T, rec *httptest.ResponseRecorder, field, message string) {
	t.Helper()
	var body effortValidationResponse
	effortRequireStatus(t, rec, http.StatusBadRequest, &body)
	assert.Equal(t, effortInvalidFieldsError, body.Error)
	for _, f := range body.Fields {
		if f.Field == field {
			assert.Equal(t, message, f.Message, "mensaje del campo %s", field)
			return
		}
	}
	t.Fatalf("el 400 no informa el campo %q; cuerpo: %s", field, rec.Body.String())
}

// requireEffortConflict exige un 409 con el código indicado y lo devuelve decodificado.
func requireEffortConflict(t *testing.T, rec *httptest.ResponseRecorder, code string) effortConflictResponse {
	t.Helper()
	var body effortConflictResponse
	effortRequireStatus(t, rec, http.StatusConflict, &body)
	assert.Equal(t, code, body.Code, "code del 409")
	assert.NotEmpty(t, body.Error, "el 409 debe llevar un mensaje para la persona")
	return body
}

// effortFixture es el "Dado" común de US1: Ana Pérez es propietaria de un proyecto En curso que
// empezó el 2026-10-01 y tiene una historia estimada y lista para planificar, solo en el backlog.
type effortFixture struct {
	env        *effortTestEnv
	ownerID    uuid.UUID
	ownerToken string
	projectID  uuid.UUID
	storyID    uuid.UUID
}

// newEffortFixture compone el entorno con las opciones recibidas y arma el "Dado" común con los
// endpoints reales.
func newEffortFixture(t *testing.T, opts ...effortEnvOption) effortFixture {
	t.Helper()
	env := newEffortTestEnv(t, opts...)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProjectInProgress(t, ownerToken, "Portal de Clientes")
	storyID := env.createReadyStory(t, ownerToken, projectID, "Alta de clientes")
	return effortFixture{env: env, ownerID: ownerID, ownerToken: ownerToken, projectID: projectID, storyID: storyID}
}

// post envía el alta de esfuerzo de Ana sobre la historia del "Dado".
func (f effortFixture) post(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return f.env.postEffort(t, f.ownerToken, f.storyID, body)
}

// requireNothingCreated verifica que la cantidad de filas de effort_entries sigue siendo la previa.
func (f effortFixture) requireNothingCreated(t *testing.T, before int64) {
	t.Helper()
	assert.Equal(t, before, f.env.effortEntryCount(t), "un rechazo no debe crear registros de esfuerzo")
}

// fridayClose es el cierre real de los sprints de los escenarios del plazo de gracia: viernes
// 2026-10-23 a las 18:00 de la zona del sistema.
func (f effortFixture) fridayClose() time.Time {
	return f.env.at(2026, time.October, 23, 18, 0)
}

// closedWithCompletedStory compromete la historia del "Dado" en un sprint Activo, la completa y
// cierra el sprint el viernes a las 18:00; devuelve el sprint.
func (f effortFixture) closedWithCompletedStory(t *testing.T) uuid.UUID {
	t.Helper()
	sprintID := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
	f.env.completeStory(t, f.ownerToken, f.storyID)
	f.env.closeSprintAt(t, f.ownerToken, sprintID, f.fridayClose())
	return sprintID
}

// requireAnchoredTo verifica en la respuesta y en la base que el registro quedó asociado al sprint
// con el estado indicado.
func (f effortFixture) requireAnchoredTo(t *testing.T, entry effortEntryResponse, sprintID uuid.UUID, status string) {
	t.Helper()
	require.NotNil(t, entry.Sprint, "el registro debe quedar asociado a un sprint; cuerpo: %v", entry.Raw)
	assert.Equal(t, sprintID.String(), entry.Sprint.ID, "sprint asociado")
	assert.Equal(t, status, entry.Sprint.Status, "estado del sprint asociado")
	stored := f.env.storedEntry(t, entry.ID)
	require.NotNil(t, stored.SprintID, "sprint_id no debe ser NULL en la base")
	assert.Equal(t, sprintID, *stored.SprintID, "sprint_id en la base")
}

// requireWithoutSprint verifica en la respuesta y en la base que el registro quedó sin sprint.
func (f effortFixture) requireWithoutSprint(t *testing.T, entry effortEntryResponse) {
	t.Helper()
	assert.Equal(t, "null", string(entry.Raw["sprint"]), "sprint debe serializarse null")
	assert.Nil(t, f.env.storedEntry(t, entry.ID).SprintID, "sprint_id debe ser NULL en la base")
}

// Escenario: Caso normal — registro de 6,5 horas sobre una historia del sprint Activo (006/US1)
func TestUS1_RegistroEnSprintActivo(t *testing.T) {
	f := newEffortFixture(t)
	sprintID := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
	before := f.env.storyHundredths(t, f.storyID)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 6.5)))

	// El registro queda atribuido a quien lo cargó, con los datos enviados y el sprint Activo.
	assert.Equal(t, f.ownerID.String(), entry.Author.ID, "autor del registro")
	assert.Equal(t, "Ana Pérez", entry.Author.FullName, "nombre del autor")
	assert.Equal(t, f.storyID.String(), entry.Story.ID, "historia del registro")
	assert.Equal(t, effortToday, entry.EntryDate)
	assert.Equal(t, effortActivity, entry.Activity)
	assert.Equal(t, "6.50", string(entry.Hours), "las horas se serializan con 2 decimales")
	assert.False(t, entry.Definitive, "un registro de un sprint Activo no es definitivo")
	f.requireAnchoredTo(t, entry, sprintID, "active")

	// Las horas reales de la historia aumentan en 6,5, leídas en la base.
	assert.Equal(t, before+650, f.env.storyHundredths(t, f.storyID), "total de la historia en centésimas")
	assert.Equal(t, f.ownerID, f.env.storedEntry(t, entry.ID).UserID, "user_id en la base")
}

// Escenario: Caso alternativo — historia solo en el backlog, sin sprint (006/US1)
func TestUS1_HistoriaSoloEnBacklog(t *testing.T) {
	f := newEffortFixture(t)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 3)))

	f.requireWithoutSprint(t, entry)
	assert.Equal(t, int64(300), f.env.storyHundredths(t, f.storyID), "las horas cuentan para la historia")
}

// Escenario: Caso alternativo — historia comprometida en un sprint Planificado (006/US1)
func TestUS1_SprintPlanificado(t *testing.T) {
	f := newEffortFixture(t)
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 4", "2026-10-26", "2026-11-06")
	f.env.assignStories(t, f.ownerToken, sprintID, f.storyID)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 2)))

	f.requireAnchoredTo(t, entry, sprintID, "planned")
}

// Escenario: Caso alternativo — historia Completada dentro de un sprint todavía Activo (006/US1)
func TestUS1_HistoriaCompletadaEnSprintActivo(t *testing.T) {
	f := newEffortFixture(t)
	sprintID := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
	f.env.completeStory(t, f.ownerToken, f.storyID)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, "Corrección de la validación del CUIT", 1.5)))

	f.requireAnchoredTo(t, entry, sprintID, "active")
}

// Escenario: Caso alternativo — segundo registro del mismo día sobre la misma historia (006/US1)
func TestUS1_SegundoRegistroDelDia(t *testing.T) {
	f := newEffortFixture(t)
	f.env.seedEffortEntry(t, f.storyID, f.ownerID, effortToday, 300, nil)

	requireEffortCreated(t, f.post(t, effortBody(effortToday, "Revisión del formulario con el equipo", 2)))

	// Quedan dos registros independientes y 5 horas de la persona en esa fecha.
	var rows int64
	require.NoError(t, f.env.db.Table("effort_entries").
		Where("story_id = ? AND user_id = ? AND entry_date = ?", f.storyID, f.ownerID, effortToday).
		Count(&rows).Error)
	assert.Equal(t, int64(2), rows, "dos registros de la persona en la historia y la fecha")
	assert.Equal(t, int64(500), f.env.userDateHundredths(t, f.ownerID, effortToday), "5 horas en la fecha")
}

// Escenario: Caso alternativo — historia "sin estimar" (006/US1)
func TestUS1_HistoriaSinEstimar(t *testing.T) {
	// Pendiente: la historia sin estimar está solo en el backlog, como la deja la 003.
	t.Run("pending", func(t *testing.T) {
		f := newEffortFixture(t)
		storyID := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes")

		entry := requireEffortCreated(t, f.env.postEffort(t, f.ownerToken, storyID, effortBody(effortToday, effortActivity, 1)))

		f.requireWithoutSprint(t, entry)
	})

	// En progreso y Completada: la historia avanza en el sprint Activo y después vuelve a "sin
	// estimar" en la base, porque esa combinación no se alcanza por la API (RN-13).
	for _, status := range []string{"in_progress", "completed"} {
		t.Run(status, func(t *testing.T) {
			f := newEffortFixture(t)
			sprintID := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
			f.env.changeStoryStatus(t, f.ownerToken, f.storyID, "in_progress")
			if status == "completed" {
				f.env.changeStoryStatus(t, f.ownerToken, f.storyID, "completed")
			}
			f.env.clearStoryPoints(t, f.storyID)

			entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 1)))

			f.requireAnchoredTo(t, entry, sprintID, "active")
		})
	}
}

// Escenario: Caso alternativo — historia Completada de un sprint cerrado, dentro del plazo de gracia (006/US1)
func TestUS1_CompletadaDentroDelPlazo(t *testing.T) {
	// Cierre el viernes a las 18:00 y alta el domingo a las 10:00, dentro del plazo de 48 horas.
	f := newEffortFixture(t)
	sprintID := f.closedWithCompletedStory(t)
	f.env.clock.Set(f.env.at(2026, time.October, 25, 10, 0))

	entry := requireEffortCreated(t, f.post(t, effortBody("2026-10-23", effortActivity, 4)))

	f.requireAnchoredTo(t, entry, sprintID, "closed")
}

// Escenario: Caso alternativo — historia devuelta al backlog al cerrarse el sprint, dentro del plazo de gracia (006/US1)
func TestUS1_DevueltaAlBacklogDentroDelPlazo(t *testing.T) {
	// La historia no se completa, vuelve al backlog al cerrarse el sprint el viernes a las 18:00 y
	// el alta llega el domingo a las 10:00, dentro del plazo de 48 horas.
	t.Run("anclado al sprint del que volvió", func(t *testing.T) {
		f := newEffortFixture(t)
		sprintID := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
		f.env.changeStoryStatus(t, f.ownerToken, f.storyID, "in_progress")
		f.env.closeSprintAt(t, f.ownerToken, sprintID, f.fridayClose())
		f.env.clock.Set(f.env.at(2026, time.October, 25, 10, 0))

		entry := requireEffortCreated(t, f.post(t, effortBody("2026-10-23", effortActivity, 4)))

		f.requireAnchoredTo(t, entry, sprintID, "closed")
	})

	// La historia no se completa en A, se recompromete en B, B se cierra dentro del plazo de A y
	// la historia vuelve otra vez al backlog: prevalece B, el de cierre más reciente (FR-014). El
	// plazo de A vence el domingo a las 18:00 y el de B, cerrado el sábado a las 12:00, el lunes a
	// las 12:00, así que el domingo a las 10:00 los dos siguen vigentes.
	t.Run("prevalece el de cierre más reciente", func(t *testing.T) {
		f := newEffortFixture(t)
		sprintA := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
		f.env.closeSprintAt(t, f.ownerToken, sprintA, f.fridayClose())
		sprintB := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 4", "2026-10-26", "2026-11-06")
		f.env.assignStories(t, f.ownerToken, sprintB, f.storyID)
		f.env.clock.Set(f.env.at(2026, time.October, 24, 10, 0))
		f.env.startSprint(t, f.ownerToken, sprintB)
		f.env.closeSprintAt(t, f.ownerToken, sprintB, f.env.at(2026, time.October, 24, 12, 0))
		f.env.clock.Set(f.env.at(2026, time.October, 25, 10, 0))

		entry := requireEffortCreated(t, f.post(t, effortBody("2026-10-23", effortActivity, 4)))

		f.requireAnchoredTo(t, entry, sprintB, "closed")
	})
}

// Escenario: Caso límite — límite del plazo de gracia el domingo a las 18:00 (006/US1)
func TestUS1_LimiteDelPlazo(t *testing.T) {
	// A las 17:59 del domingo faltan un minuto para las 48 horas: el registro se acepta.
	t.Run("17:59 aceptado", func(t *testing.T) {
		f := newEffortFixture(t)
		sprintID := f.closedWithCompletedStory(t)
		f.env.clock.Set(f.env.at(2026, time.October, 25, 17, 59))

		entry := requireEffortCreated(t, f.post(t, effortBody("2026-10-23", effortActivity, 2)))

		f.requireAnchoredTo(t, entry, sprintID, "closed")
	})

	// A las 18:01 el plazo ya venció y la historia Completada sigue en el sprint: se rechaza.
	t.Run("18:01 rechazado", func(t *testing.T) {
		f := newEffortFixture(t)
		f.closedWithCompletedStory(t)
		f.env.clock.Set(f.env.at(2026, time.October, 25, 18, 1))
		before := f.env.effortEntryCount(t)

		body := requireEffortConflict(t, f.post(t, effortBody("2026-10-23", effortActivity, 2)), "effort_finalized")

		assert.Equal(t, "null", string(body.AvailableHours), "availableHours es null fuera del tope diario")
		f.requireNothingCreated(t, before)
	})
}

// Escenario: Caso límite — exactamente 24 horas en una fecha sin horas cargadas (006/US1)
func TestUS1_VeinticuatroHorasExactas(t *testing.T) {
	f := newEffortFixture(t)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 24)))

	assert.Equal(t, "24.00", string(entry.Hours))
	assert.Equal(t, int64(2400), f.env.userDateHundredths(t, f.ownerID, effortToday))
}

// Escenario: Caso límite — 0,25 horas, el mínimo de la escala (006/US1)
func TestUS1_CuartoDeHora(t *testing.T) {
	f := newEffortFixture(t)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 0.25)))

	assert.Equal(t, "0.25", string(entry.Hours))
	assert.Equal(t, int64(25), f.env.storyHundredths(t, f.storyID))
}

// Escenario: Caso límite — actividad de 300 y de 301 caracteres (006/US1)
func TestUS1_ActividadDeTrescientosCaracteres(t *testing.T) {
	// Con acentos, para que la longitud se cuente en caracteres y no en bytes.
	t.Run("300 aceptada", func(t *testing.T) {
		f := newEffortFixture(t)
		activity := strings.Repeat("ñ", 300)

		entry := requireEffortCreated(t, f.post(t, effortBody(effortToday, activity, 1)))

		assert.Equal(t, activity, entry.Activity)
		assert.Equal(t, 300, utf8.RuneCountInString(f.env.storedEntry(t, entry.ID).Activity))
	})

	t.Run("301 rechazada", func(t *testing.T) {
		f := newEffortFixture(t)
		before := f.env.effortEntryCount(t)

		rec := f.post(t, effortBody(effortToday, strings.Repeat("ñ", 301), 1))

		requireEffortFieldRejected(t, rec, "activity", validationMsgActivityLong)
		f.requireNothingCreated(t, before)
	})
}

// Escenario: Caso límite — fecha igual al día de inicio del proyecto (006/US1)
func TestUS1_DiaDeInicioDelProyecto(t *testing.T) {
	f := newEffortFixture(t)

	entry := requireEffortCreated(t, f.post(t, effortBody(effortProjectStart, effortActivity, 3)))

	assert.Equal(t, effortProjectStart, entry.EntryDate)
}

// Escenario: Caso límite — la fecha de hoy según la zona horaria del sistema (006/US1)
func TestUS1_HoySegunZonaDelSistema(t *testing.T) {
	f := newEffortFixture(t)

	// Las 23:30 del 2026-10-21 en la zona del sistema son las 02:30 del 2026-10-22 en UTC.
	f.env.clock.Set(f.env.at(2026, time.October, 21, 23, 30))
	require.Equal(t, "2026-10-22", f.env.clock.Now().UTC().Format(time.DateOnly), "precondición: en UTC ya es el día siguiente")

	entry := requireEffortCreated(t, f.post(t, effortBody("2026-10-21", effortActivity, 2)))
	assert.Equal(t, "2026-10-21", entry.EntryDate)

	// La fecha de UTC todavía es futura en la zona del sistema, así que se rechaza.
	before := f.env.effortEntryCount(t)
	requireEffortFieldRejected(t, f.post(t, effortBody("2026-10-22", effortActivity, 2)), "entryDate", effortMsgDateInFuture)
	f.requireNothingCreated(t, before)
}

// effortThreeProjects arma el "Dado" del tope entre proyectos: además del proyecto del fixture, Ana
// tiene otros dos proyectos En curso, cada uno con una historia; en los dos primeros ya registró
// 12 y 8 horas hoy. Devuelve la historia del tercer proyecto y los nombres de los tres proyectos.
func effortThreeProjects(t *testing.T, f effortFixture) (uuid.UUID, []string) {
	t.Helper()
	secondProject := f.env.createProjectInProgress(t, f.ownerToken, "Mesa de Ayuda")
	secondStory := f.env.createReadyStory(t, f.ownerToken, secondProject, "Alta de tickets")
	thirdProject := f.env.createProjectInProgress(t, f.ownerToken, "Facturación Electrónica")
	thirdStory := f.env.createReadyStory(t, f.ownerToken, thirdProject, "Emisión de facturas")
	f.env.seedEffortEntry(t, f.storyID, f.ownerID, effortToday, 1200, nil)
	f.env.seedEffortEntry(t, secondStory, f.ownerID, effortToday, 800, nil)
	return thirdStory, []string{"Portal de Clientes", "Mesa de Ayuda", "Facturación Electrónica"}
}

// Escenario: Caso límite — 20 horas en dos proyectos más 4 en un tercero llegan justo al tope (006/US1)
func TestUS1_TopeEntreProyectos(t *testing.T) {
	f := newEffortFixture(t)
	thirdStory, _ := effortThreeProjects(t, f)

	requireEffortCreated(t, f.env.postEffort(t, f.ownerToken, thirdStory, effortBody(effortToday, effortActivity, 4)))

	assert.Equal(t, int64(2400), f.env.userDateHundredths(t, f.ownerID, effortToday), "el día llega justo a 24 horas")
}

// Escenario: Caso de error — tope diario superado (006/US1)
func TestUS1_TopeDiarioSuperado(t *testing.T) {
	f := newEffortFixture(t)
	thirdStory, projectNames := effortThreeProjects(t, f)
	before := f.env.effortEntryCount(t)

	rec := f.env.postEffort(t, f.ownerToken, thirdStory, effortBody(effortToday, effortActivity, 5))

	// El rechazo informa las 4 horas disponibles con 2 decimales y no nombra ningún proyecto.
	body := requireEffortConflict(t, rec, "daily_cap_exceeded")
	assert.Equal(t, "4.00", string(body.AvailableHours), "availableHours")
	for _, name := range projectNames {
		assert.NotContains(t, body.Error, name, "el mensaje del tope no nombra proyectos")
	}
	f.requireNothingCreated(t, before)
	assert.Equal(t, int64(2000), f.env.userDateHundredths(t, f.ownerID, effortToday), "el día sigue con 20 horas")
}

// Escenario: Caso de error — horas en 0, negativas o mayores que 24 (006/US1)
func TestUS1_HorasFueraDeRango(t *testing.T) {
	cases := []struct {
		name  string
		hours float64
	}{
		{name: "cero", hours: 0},
		{name: "negativo", hours: -1},
		{name: "24,25", hours: 24.25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEffortFixture(t)
			before := f.env.effortEntryCount(t)

			rec := f.post(t, effortBody(effortToday, effortActivity, tc.hours))

			requireEffortFieldRejected(t, rec, "hours", validationMsgHoursRange)
			f.requireNothingCreated(t, before)
		})
	}
}

// Escenario: Caso de error — horas que no son múltiplo de 0,25 (006/US1)
func TestUS1_HorasNoMultiplo(t *testing.T) {
	f := newEffortFixture(t)
	before := f.env.effortEntryCount(t)

	rec := f.post(t, effortBody(effortToday, effortActivity, 1.3))

	requireEffortFieldRejected(t, rec, "hours", validationMsgHoursMultiple)
	f.requireNothingCreated(t, before)
}

// Escenario: Caso de error — actividad ausente o compuesta solo por espacios (006/US1)
func TestUS1_ActividadVacia(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{name: "ausente", body: map[string]any{"entryDate": effortToday, "hours": 2}},
		{name: "espacios", body: effortBody(effortToday, "   ", 2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEffortFixture(t)
			before := f.env.effortEntryCount(t)

			rec := f.post(t, tc.body)

			requireEffortFieldRejected(t, rec, "activity", validationMsgActivityEmpty)
			f.requireNothingCreated(t, before)
		})
	}
}

// Escenario: Caso de error — fecha de mañana (006/US1)
func TestUS1_FechaFutura(t *testing.T) {
	f := newEffortFixture(t)
	before := f.env.effortEntryCount(t)

	rec := f.post(t, effortBody("2026-10-22", effortActivity, 2))

	requireEffortFieldRejected(t, rec, "entryDate", effortMsgDateInFuture)
	f.requireNothingCreated(t, before)
}

// Escenario: Caso de error — fecha anterior al inicio del proyecto (006/US1)
func TestUS1_FechaAnteriorAlInicio(t *testing.T) {
	f := newEffortFixture(t)
	before := f.env.effortEntryCount(t)

	rec := f.post(t, effortBody("2026-09-30", effortActivity, 2))

	requireEffortFieldRejected(t, rec, "entryDate", effortMsgDateBeforeStart)
	f.requireNothingCreated(t, before)
}

// Escenario: Caso de error — historia Completada de un sprint cerrado hace cinco días (006/US1)
func TestUS1_SprintDefinitivo(t *testing.T) {
	f := newEffortFixture(t)
	f.closedWithCompletedStory(t)
	f.env.clock.Advance(5 * 24 * time.Hour)
	before := f.env.effortEntryCount(t)

	body := requireEffortConflict(t, f.post(t, effortBody("2026-10-23", effortActivity, 2)), "effort_finalized")

	assert.Equal(t, "null", string(body.AvailableHours), "availableHours es null fuera del tope diario")
	f.requireNothingCreated(t, before)
}

// Escenario: Caso de error — proyecto Finalizado con el plazo de gracia de su último sprint vigente (006/US1)
func TestUS1_ProyectoFinalizado(t *testing.T) {
	f := newEffortFixture(t)
	f.closedWithCompletedStory(t)

	// El sábado a las 10:00 el plazo de 48 horas del sprint sigue vigente; el proyecto se finaliza.
	f.env.clock.Set(f.env.at(2026, time.October, 24, 10, 0))
	f.env.setProjectStatus(t, f.ownerToken, f.projectID, "finished")
	before := f.env.effortEntryCount(t)

	body := requireEffortConflict(t, f.post(t, effortBody("2026-10-23", effortActivity, 2)), "read_only")

	assert.Equal(t, effortMsgReadOnly, body.Error)
	assert.Equal(t, "null", string(body.AvailableHours), "availableHours es null fuera del tope diario")
	f.requireNothingCreated(t, before)
}

// Escenario: Caso de error — usuario que no es integrante del proyecto (006/US1)
func TestUS1_NoIntegrante(t *testing.T) {
	f := newEffortFixture(t)
	_, outsiderToken := f.env.signUp(t, "Carlos Gómez", "carlos@example.com")
	storyNotFound := f.env.do(t, http.MethodGet, "/api/v1/stories/"+f.storyID.String(), outsiderToken, nil)
	require.Equal(t, http.StatusNotFound, storyNotFound.Code, "precondición: la 003 responde 404 a quien no es integrante")
	before := f.env.effortEntryCount(t)

	rec := f.env.postEffort(t, outsiderToken, f.storyID, effortBody(effortToday, effortActivity, 2))

	// La respuesta es el mismo 404 de historia inexistente de la 003.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, storyNotFound.Body.String(), rec.Body.String(), "el cuerpo debe ser el 404 de la 003")
	f.requireNothingCreated(t, before)
}

// Escenario: Caso de error — intento de indicar a otra persona como autor (006/US1)
func TestUS1_AutorAjenoIgnorado(t *testing.T) {
	f := newEffortFixture(t)
	brunoID, brunoToken := f.env.signUp(t, "Bruno Díaz", "bruno@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "bruno@example.com")
	body := effortBody(effortToday, effortActivity, 2)
	body["userId"] = f.ownerID.String()

	entry := requireEffortCreated(t, f.env.postEffort(t, brunoToken, f.storyID, body))

	// El registro queda a nombre del titular de la sesión, nunca de la persona indicada.
	assert.Equal(t, brunoID.String(), entry.Author.ID, "autor del registro")
	assert.Equal(t, brunoID, f.env.storedEntry(t, entry.ID).UserID, "user_id en la base")
	assert.Equal(t, int64(0), f.env.userDateHundredths(t, f.ownerID, effortToday), "nada a nombre de la persona indicada")
}

// Cuerpos exactos de los 409 de eliminación de la 003 y la 004 que deben aparecer cuando los puertos
// de esfuerzo leen los registros reales (data-model.md sección 7 de cada feature).
const (
	effortPortsSprintHasEffortMessage = "El sprint tiene esfuerzo registrado. Primero hay que eliminar esos registros."
	effortPortsStoryBlockedMessage    = "La historia tiene historial asociado y no se puede eliminar."
	effortPortsSprintHistoryMessage   = "Estuvo asignada a un sprint."
	effortPortsEffortMessage          = "Tiene esfuerzo registrado."
)

// TestEffortPorts_BloqueanEliminaciones verifica de punta a punta los puertos de esfuerzo de la 003
// y la 004 (T031), sin etiqueta de escenario porque no corresponde a uno de spec.md sino a RC-05,
// RC-07 y RN-22. Con los endpoints reales, Ana compromete una historia en un sprint Planificado y le
// registra esfuerzo: eliminar el sprint se rechaza con el 409 de esfuerzo de la 004 y el sprint sigue
// en la base. Después se borran todos los registros de la historia y eliminarla sigue respondiendo el
// 409 de la 003, que incluye el motivo effort además de sprint_history, porque la historia estuvo
// comprometida en el sprint.
func TestEffortPorts_BloqueanEliminaciones(t *testing.T) {
	f := newEffortFixture(t)
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 4", "2026-10-26", "2026-11-06")
	f.env.assignStories(t, f.ownerToken, sprintID, f.storyID)
	requireEffortCreated(t, f.post(t, effortBody(effortToday, effortActivity, 2)))

	// RC-07: el sprint Planificado con esfuerzo asociado no se elimina y sigue en la base.
	rec := f.env.do(t, http.MethodDelete, "/api/v1/sprints/"+sprintID.String(), f.ownerToken, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "DELETE del sprint; cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+effortPortsSprintHasEffortMessage+`"}`, rec.Body.String())
	var sprints int64
	require.NoError(t, f.env.db.Table("sprints").Where("id = ?", sprintID).Count(&sprints).Error)
	assert.EqualValues(t, 1, sprints, "el sprint con esfuerzo debe seguir en la base")

	// Desviación deliberada: los registros se borran directamente en la base de test porque el
	// endpoint de eliminación de esfuerzo es de la US3 (T050) y todavía no existe. Cuando exista, este
	// bloque se reemplaza por el DELETE real de cada registro de la historia.
	require.NoError(t, f.env.db.Exec("DELETE FROM effort_entries WHERE story_id = ?", f.storyID).Error,
		"no se pudieron borrar los registros de la historia")
	require.Zero(t, f.env.storyHundredths(t, f.storyID), "precondición: la historia ya no tiene registros")

	// RC-05 y RN-22: sin registros, la historia sigue sin poder eliminarse por el motivo effort.
	rec = f.env.do(t, http.MethodDelete, "/api/v1/stories/"+f.storyID.String(), f.ownerToken, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "DELETE de la historia; cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+effortPortsStoryBlockedMessage+`","reasons":[`+
		`{"code":"sprint_history","message":"`+effortPortsSprintHistoryMessage+`"},`+
		`{"code":"effort","message":"`+effortPortsEffortMessage+`"}]}`, rec.Body.String())
}

// Pruebas de integración de US2 (T034): GET /api/v1/stories/{storyId}/effort con un caso por
// escenario de spec.md. Los registros se siembran en la base con su fecha, su instante de creación y,
// cuando hace falta, su identificador, para verificar sin depender del reloj real el orden de FR-035
// (entry_date DESC, created_at ASC, id ASC) y el del desglose (full_name ASC, user_id ASC). Los
// proyectos, los integrantes, las historias y los sprints se preparan con los endpoints reales.

// effortMemberResponse refleja MemberEffort del contrato; hours se conserva como texto literal para
// comprobar los 2 decimales.
type effortMemberResponse struct {
	User  effortUserRef   `json:"user"`
	Hours json.RawMessage `json:"hours"`
}

// storyEffortResponse refleja StoryEffort del contrato. Raw guarda el cuerpo para distinguir una
// lista vacía de null.
type storyEffortResponse struct {
	StoryID    string                     `json:"storyId"`
	TotalHours json.RawMessage            `json:"totalHours"`
	ByMember   []effortMemberResponse     `json:"byMember"`
	Entries    []effortEntryResponse      `json:"entries"`
	Raw        map[string]json.RawMessage `json:"-"`
}

// storyEffortPath es la ruta de la consulta del esfuerzo de una historia según contracts/openapi.yaml.
func storyEffortPath(storyID string) string {
	return "/api/v1/stories/" + storyID + "/effort"
}

// getStoryEffort consulta el esfuerzo de la historia con el token indicado, exige un 200 y devuelve el
// cuerpo decodificado.
func (e *effortTestEnv) getStoryEffort(t *testing.T, token string, storyID uuid.UUID) storyEffortResponse {
	t.Helper()
	rec := e.do(t, http.MethodGet, storyEffortPath(storyID.String()), token, nil)
	var body storyEffortResponse
	effortRequireStatus(t, rec, http.StatusOK, &body)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body.Raw), "el cuerpo del 200 no es un objeto JSON")
	return body
}

// effortSeed describe un registro sembrado para US2: el identificador es opcional (uuid.Nil genera
// uno nuevo) y el instante de creación se fija para controlar el orden.
type effortSeed struct {
	id        uuid.UUID
	userID    uuid.UUID
	sprintID  *uuid.UUID
	entryDate string
	hours     int
	activity  string
	createdAt time.Time
}

// seedStoryEntry inserta en la base el registro descrito sobre la historia y devuelve su
// identificador. CreatedAt y UpdatedAt se fijan explícitamente, así GORM no los reemplaza.
func (e *effortTestEnv) seedStoryEntry(t *testing.T, storyID uuid.UUID, seed effortSeed) uuid.UUID {
	t.Helper()
	id := seed.id
	if id == uuid.Nil {
		id = uuid.New()
	}
	activity := seed.activity
	if activity == "" {
		activity = "Horas cargadas previamente"
	}
	entry := domain.EffortEntry{
		ID:              id,
		StoryID:         storyID,
		UserID:          seed.userID,
		SprintID:        seed.sprintID,
		EntryDate:       effortDate(t, seed.entryDate),
		Activity:        activity,
		HoursHundredths: seed.hours,
		CreatedAt:       seed.createdAt,
		UpdatedAt:       seed.createdAt,
	}
	require.NoError(t, e.db.Create(&entry).Error, "no se pudo sembrar el registro de esfuerzo")
	return id
}

// effortSeedBase es el instante de creación de referencia de los registros sembrados.
var effortSeedBase = time.Date(2026, time.October, 21, 9, 0, 0, 0, time.UTC)

// effortEntryIDs devuelve los identificadores de los registros en el orden de la respuesta.
func effortEntryIDs(entries []effortEntryResponse) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

// requireStoryNotFoundLike03 exige que la respuesta sea exactamente el 404 de historia inexistente
// que la 003 da para la misma historia con el mismo token, sin datos de esfuerzo.
func (e *effortTestEnv) requireStoryNotFoundLike03(t *testing.T, token, storyID string, rec *httptest.ResponseRecorder) {
	t.Helper()
	storyNotFound := e.do(t, http.MethodGet, "/api/v1/stories/"+storyID, token, nil)
	require.Equal(t, http.StatusNotFound, storyNotFound.Code, "precondición: la 003 responde 404")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, storyNotFound.Body.String(), rec.Body.String(), "el cuerpo debe ser el 404 de la 003")
	assert.NotContains(t, rec.Body.String(), "totalHours", "el 404 no revela datos de esfuerzo")
	assert.NotContains(t, rec.Body.String(), "entries", "el 404 no revela registros")
}

// Escenario: Caso normal — total de 16 horas y desglose de 10 y 6 horas por integrante (006/US2)
func TestUS2_TotalYDesglose(t *testing.T) {
	// Bruno Díaz carga 10 horas en tres registros y Ana Pérez 6 en dos. Las fechas, los instantes
	// de creación y los identificadores se eligen para que cada criterio de orden decida al menos un
	// par: la fecha descendente, el created_at ascendente a igual fecha y el id ascendente a igual
	// fecha e instante. Se siembran desordenados para no depender del orden de inserción.
	t.Run("total, desglose por nombre y registros en orden", func(t *testing.T) {
		f := newEffortFixture(t)
		brunoID, brunoToken := f.env.signUp(t, "Bruno Díaz", "bruno@example.com")
		f.env.addMember(t, f.ownerToken, f.projectID, "bruno@example.com")
		lowID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
		highID := uuid.MustParse("ffffffff-ffff-4fff-bfff-ffffffffffff")

		e4 := f.env.seedStoryEntry(t, f.storyID, effortSeed{id: highID, userID: f.ownerID, entryDate: "2026-10-15", hours: 400, createdAt: effortSeedBase})
		e5 := f.env.seedStoryEntry(t, f.storyID, effortSeed{id: lowID, userID: brunoID, entryDate: "2026-10-15", hours: 300, createdAt: effortSeedBase})
		e1 := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: brunoID, entryDate: "2026-10-20", hours: 400, createdAt: effortSeedBase.Add(time.Hour)})
		e2 := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-20", hours: 200, createdAt: effortSeedBase})
		e3 := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: brunoID, entryDate: "2026-10-21", hours: 300, activity: effortActivity, createdAt: effortSeedBase.Add(2 * time.Hour)})

		body := f.env.getStoryEffort(t, brunoToken, f.storyID)

		// Total y desglose: Ana va primero por nombre aunque tenga menos horas.
		assert.Equal(t, f.storyID.String(), body.StoryID, "storyId")
		assert.Equal(t, "16.00", string(body.TotalHours), "totalHours con 2 decimales")
		require.Len(t, body.ByMember, 2, "una línea por persona")
		assert.Equal(t, effortUserRef{ID: f.ownerID.String(), FullName: "Ana Pérez"}, body.ByMember[0].User)
		assert.Equal(t, "6.00", string(body.ByMember[0].Hours))
		assert.Equal(t, effortUserRef{ID: brunoID.String(), FullName: "Bruno Díaz"}, body.ByMember[1].User)
		assert.Equal(t, "10.00", string(body.ByMember[1].Hours))

		// Registros en el orden de FR-035.
		require.Len(t, body.Entries, 5, "cinco registros")
		assert.Equal(t, []string{e3.String(), e2.String(), e1.String(), e5.String(), e4.String()},
			effortEntryIDs(body.Entries), "orden entry_date DESC, created_at ASC, id ASC")

		// Cada registro lleva fecha, autor, actividad, horas, historia y sprint (FR-033).
		first := body.Entries[0]
		assert.Equal(t, "2026-10-21", first.EntryDate)
		assert.Equal(t, effortActivity, first.Activity)
		assert.Equal(t, "3.00", string(first.Hours))
		assert.Equal(t, effortUserRef{ID: brunoID.String(), FullName: "Bruno Díaz"}, first.Author)
		assert.Equal(t, effortStoryRef{ID: f.storyID.String(), Title: "Alta de clientes"}, first.Story)
		assert.Nil(t, first.Sprint, "la historia solo está en el backlog: sin sprint")
		assert.False(t, first.Definitive)
		assert.Equal(t, "2026-10-15", body.Entries[4].EntryDate)
		assert.Equal(t, "4.00", string(body.Entries[4].Hours))
		assert.Equal(t, "Ana Pérez", body.Entries[4].Author.FullName)
	})

	// Dos personas con el mismo nombre completo: el desglose desempata por user_id ascendente y
	// mantiene una línea para cada una.
	t.Run("desempate del desglose por user_id", func(t *testing.T) {
		f := newEffortFixture(t)
		twinID, _ := f.env.signUp(t, "Ana Pérez", "ana.perez@example.com")
		f.env.addMember(t, f.ownerToken, f.projectID, "ana.perez@example.com")
		f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-20", hours: 100, createdAt: effortSeedBase})
		f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: twinID, entryDate: "2026-10-20", hours: 200, createdAt: effortSeedBase})
		lower, higher := f.ownerID, twinID
		lowerHours, higherHours := "1.00", "2.00"
		if higher.String() < lower.String() {
			lower, higher = higher, lower
			lowerHours, higherHours = higherHours, lowerHours
		}

		body := f.env.getStoryEffort(t, f.ownerToken, f.storyID)

		require.Len(t, body.ByMember, 2, "una línea por persona aunque compartan el nombre")
		assert.Equal(t, lower.String(), body.ByMember[0].User.ID, "primero el user_id menor")
		assert.Equal(t, lowerHours, string(body.ByMember[0].Hours))
		assert.Equal(t, higher.String(), body.ByMember[1].User.ID)
		assert.Equal(t, higherHours, string(body.ByMember[1].Hours))
		assert.Equal(t, "3.00", string(body.TotalHours))
	})
}

// Escenario: Caso alternativo — registros de dos sprints distintos y uno sin sprint (006/US2)
func TestUS2_RegistrosDeVariosSprints(t *testing.T) {
	// La historia pasa por el Sprint 3, que se cierra el viernes sin completarla, y queda
	// comprometida en el Sprint 4 Planificado. Cinco días después del cierre, el plazo de gracia del
	// Sprint 3 venció: su registro es definitivo y los demás no.
	f := newEffortFixture(t)
	sprintA := f.env.activeSprintWith(t, f.ownerToken, f.projectID, f.storyID)
	f.env.closeSprintAt(t, f.ownerToken, sprintA, f.fridayClose())
	sprintB := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 4", "2026-10-26", "2026-11-06")
	f.env.assignStories(t, f.ownerToken, sprintB, f.storyID)

	inB := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, sprintID: &sprintB, entryDate: "2026-10-23", hours: 175, createdAt: effortSeedBase})
	inA := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, sprintID: &sprintA, entryDate: "2026-10-22", hours: 250, createdAt: effortSeedBase})
	noSprint := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-05", hours: 100, createdAt: effortSeedBase})
	f.env.clock.Set(f.fridayClose().Add(5 * 24 * time.Hour))

	body := f.env.getStoryEffort(t, f.ownerToken, f.storyID)

	// El total incluye los tres registros, también el sin sprint.
	assert.Equal(t, "5.25", string(body.TotalHours), "el total incluye todos los registros")
	require.Len(t, body.Entries, 3)
	assert.Equal(t, []string{inB.String(), inA.String(), noSprint.String()}, effortEntryIDs(body.Entries))

	// Cada registro con su sprint asociado; el sin sprint, con sprint: null.
	require.NotNil(t, body.Entries[0].Sprint, "registro del Sprint 4")
	assert.Equal(t, effortSprintRef{ID: sprintB.String(), Name: "Sprint 4", Status: "planned"}, *body.Entries[0].Sprint)
	assert.False(t, body.Entries[0].Definitive, "sprint Planificado: no definitivo")
	require.NotNil(t, body.Entries[1].Sprint, "registro del Sprint 3")
	assert.Equal(t, effortSprintRef{ID: sprintA.String(), Name: "Sprint 3", Status: "closed"}, *body.Entries[1].Sprint)
	assert.True(t, body.Entries[1].Definitive, "sprint cerrado con el plazo vencido: definitivo")
	assert.Nil(t, body.Entries[2].Sprint, "registro sin sprint")
	assert.False(t, body.Entries[2].Definitive, "sin sprint: no definitivo")

	var rawEntries []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body.Raw["entries"], &rawEntries))
	assert.Equal(t, "null", string(rawEntries[2]["sprint"]), "sprint debe serializarse null")
}

// Escenario: Caso alternativo — registros de una persona quitada del proyecto (006/US2)
func TestUS2_ExIntegrante(t *testing.T) {
	// Bruno carga 3 horas y después la propietaria lo quita del proyecto con la 002.
	f := newEffortFixture(t)
	brunoID, _ := f.env.signUp(t, "Bruno Díaz", "bruno@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "bruno@example.com")
	brunoEntry := f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: brunoID, entryDate: "2026-10-20", hours: 300, createdAt: effortSeedBase})
	f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-21", hours: 200, createdAt: effortSeedBase})
	effortRequireStatus(t, f.env.do(t, http.MethodDelete,
		"/api/v1/projects/"+f.projectID.String()+"/members/"+brunoID.String(), f.ownerToken, nil), http.StatusNoContent, nil)

	body := f.env.getStoryEffort(t, f.ownerToken, f.storyID)

	// Sus registros siguen visibles con su nombre y cuentan en el total y en el desglose.
	assert.Equal(t, "5.00", string(body.TotalHours), "el total incluye las horas del ex integrante")
	require.Len(t, body.ByMember, 2)
	assert.Equal(t, effortUserRef{ID: brunoID.String(), FullName: "Bruno Díaz"}, body.ByMember[1].User)
	assert.Equal(t, "3.00", string(body.ByMember[1].Hours))
	require.Len(t, body.Entries, 2)
	assert.Equal(t, brunoEntry.String(), body.Entries[1].ID)
	assert.Equal(t, effortUserRef{ID: brunoID.String(), FullName: "Bruno Díaz"}, body.Entries[1].Author)
}

// Escenario: Caso límite — historia sin ningún registro de esfuerzo (006/US2)
func TestUS2_HistoriaSinRegistros(t *testing.T) {
	f := newEffortFixture(t)

	body := f.env.getStoryEffort(t, f.ownerToken, f.storyID)

	// Total 0.00 y listas vacías, nunca null: no es un error.
	assert.Equal(t, f.storyID.String(), body.StoryID)
	assert.Equal(t, "0.00", string(body.TotalHours), "totalHours")
	assert.Equal(t, "[]", string(body.Raw["byMember"]), "byMember debe ser [] y no null")
	assert.Equal(t, "[]", string(body.Raw["entries"]), "entries debe ser [] y no null")
}

// Escenario: Caso límite — un único registro de 0,25 horas (006/US2)
func TestUS2_CuartoDeHora(t *testing.T) {
	f := newEffortFixture(t)
	f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-20", hours: 25, createdAt: effortSeedBase})

	body := f.env.getStoryEffort(t, f.ownerToken, f.storyID)

	// El cuarto de hora se conserva sin pérdida de precisión en el total, el desglose y el registro.
	assert.Equal(t, "0.25", string(body.TotalHours), "totalHours")
	require.Len(t, body.ByMember, 1)
	assert.Equal(t, "0.25", string(body.ByMember[0].Hours))
	require.Len(t, body.Entries, 1)
	assert.Equal(t, "0.25", string(body.Entries[0].Hours))
}

// Escenario: Caso límite — un único integrante con registros en tres fechas distintas (006/US2)
func TestUS2_UnaLineaPorPersona(t *testing.T) {
	f := newEffortFixture(t)
	f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-14", hours: 100, createdAt: effortSeedBase})
	f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-21", hours: 250, createdAt: effortSeedBase})
	f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-17", hours: 325, createdAt: effortSeedBase})

	body := f.env.getStoryEffort(t, f.ownerToken, f.storyID)

	// Una sola línea con la suma de las tres fechas y las tres fechas por separado en la lista.
	require.Len(t, body.ByMember, 1, "una sola línea para la persona")
	assert.Equal(t, effortUserRef{ID: f.ownerID.String(), FullName: "Ana Pérez"}, body.ByMember[0].User)
	assert.Equal(t, "6.75", string(body.ByMember[0].Hours))
	assert.Equal(t, "6.75", string(body.TotalHours))
	require.Len(t, body.Entries, 3, "tres registros")
	dates := []string{body.Entries[0].EntryDate, body.Entries[1].EntryDate, body.Entries[2].EntryDate}
	assert.Equal(t, []string{"2026-10-21", "2026-10-17", "2026-10-14"}, dates, "fecha descendente")
}

// Escenario: Caso de error — usuario que no es integrante del proyecto (006/US2)
func TestUS2_NoIntegrante(t *testing.T) {
	f := newEffortFixture(t)
	f.env.seedStoryEntry(t, f.storyID, effortSeed{userID: f.ownerID, entryDate: "2026-10-20", hours: 300, createdAt: effortSeedBase})
	_, outsiderToken := f.env.signUp(t, "Carlos Gómez", "carlos@example.com")

	rec := f.env.do(t, http.MethodGet, storyEffortPath(f.storyID.String()), outsiderToken, nil)

	// La respuesta es el mismo 404 de historia inexistente de la 003 y no revela ningún dato.
	f.env.requireStoryNotFoundLike03(t, outsiderToken, f.storyID.String(), rec)
}

// Escenario: Caso de error — identificador de historia que no existe (006/US2)
func TestUS2_HistoriaInexistente(t *testing.T) {
	// Un UUID válido sin historia y un identificador mal formado reciben el mismo 404 de la 003.
	cases := []struct {
		name    string
		storyID string
	}{
		{name: "UUID sin historia", storyID: uuid.NewString()},
		{name: "identificador mal formado", storyID: "no-es-un-uuid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newEffortFixture(t)

			rec := f.env.do(t, http.MethodGet, storyEffortPath(tc.storyID), f.ownerToken, nil)

			f.env.requireStoryNotFoundLike03(t, f.ownerToken, tc.storyID, rec)
		})
	}
}
