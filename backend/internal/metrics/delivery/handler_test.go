package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cuerpos exactos de los 404 que la 002 y la 004 devuelven, y que las métricas deben reutilizar
// (FR-042, SC-014).
const (
	sprintNotFoundBody  = `{"error":"El sprint no existe."}`
	projectNotFoundBody = `{"error":"El proyecto no existe."}`
)

// notCalculable marca, en las comparaciones de valores, una métrica con status not_calculable y
// value null.
const notCalculable = "not_calculable"

// Formatos exigidos por FR-033 y FR-034: Story Points y conteos enteros; horas y porcentajes con
// exactamente 2 decimales.
var (
	integerFormat = regexp.MustCompile(`^\d+$`)
	decimalFormat = regexp.MustCompile(`^-?\d+\.\d{2}$`)
)

// metricValue es una métrica de la respuesta; value queda como texto JSON para verificar su formato
// exacto (por ejemplo "126.00" y no 126).
type metricValue struct {
	Status   string          `json:"status"`
	Value    json.RawMessage `json:"value"`
	Finality *string         `json:"finality"`
}

// sprintMetricsBody es la respuesta de GET /metrics/sprints/{sprintId} (contracts/openapi.yaml).
type sprintMetricsBody struct {
	Sprint struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Status    string  `json:"status"`
		StartDate string  `json:"startDate"`
		EndDate   string  `json:"endDate"`
		ClosedAt  *string `json:"closedAt"`
	} `json:"sprint"`
	PlannedStoryPoints     metricValue     `json:"plannedStoryPoints"`
	CompletedStoryPoints   metricValue     `json:"completedStoryPoints"`
	EstimatedHours         metricValue     `json:"estimatedHours"`
	ActualHours            metricValue     `json:"actualHours"`
	DeviationHours         metricValue     `json:"deviationHours"`
	DeviationPercent       metricValue     `json:"deviationPercent"`
	StoryCompletionPercent metricValue     `json:"storyCompletionPercent"`
	DefectsDetected        metricValue     `json:"defectsDetected"`
	DefectsResolved        metricValue     `json:"defectsResolved"`
	Warnings               json.RawMessage `json:"warnings"`
}

// metricsFixture es el "Dado" común: una persona con sesión, un proyecto En curso con factor 6 y el
// entorno con el que se arman sus sprints. Se arma con los endpoints reales de las otras features.
type metricsFixture struct {
	env        *metricsTestEnv
	ownerToken string
	projectID  uuid.UUID
}

// metricsProjectName es el nombre del proyecto de prueba, que también se usa para cambiar su factor.
const metricsProjectName = "Portal de Clientes"

// newMetricsFixture compone el entorno, registra a Ana Pérez y le crea un proyecto con factor 6.
func newMetricsFixture(t *testing.T) metricsFixture {
	t.Helper()
	env := newMetricsTestEnv(t)
	_, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProject(t, ownerToken, metricsProjectName, 6)
	return metricsFixture{env: env, ownerToken: ownerToken, projectID: projectID}
}

// fridayClose es el cierre real de los sprints de prueba: viernes 2026-10-23 a las 18:00 de la zona
// del sistema.
func (f metricsFixture) fridayClose() time.Time {
	return f.env.at(2026, time.October, 23, 18, 0)
}

// closedSprint arma, con las otras features, un sprint del 2026-10-12 al 2026-10-23 con una historia
// por cada valor de points, lo inicia, completa las primeras `completed` historias, carga `hours`
// horas sobre la primera historia y lo cierra el viernes a las 18:00. Devuelve el sprint y sus
// historias en el orden de points.
func (f metricsFixture) closedSprint(t *testing.T, points []int, completed int, hours float64) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	stories := make([]uuid.UUID, 0, len(points))
	for i, p := range points {
		stories = append(stories, f.env.createStory(t, f.ownerToken, f.projectID, "Historia "+string(rune('A'+i)), p))
	}
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-12", "2026-10-23", stories...)
	f.env.startSprint(t, f.ownerToken, sprintID)
	for _, storyID := range stories[:completed] {
		f.env.completeStory(t, f.ownerToken, storyID)
	}
	if hours > 0 {
		f.env.logHours(t, f.ownerToken, stories[0], hours)
	}
	f.env.closeSprintAt(t, f.ownerToken, sprintID, f.fridayClose())
	return sprintID, stories
}

// normalSprint arma el sprint Cerrado de US1-1: cuatro historias de 8, 5, 5 y 3 Story Points (21), las
// dos primeras completadas (13), factor 6 y las horas reales indicadas.
func (f metricsFixture) normalSprint(t *testing.T, hours float64) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	return f.closedSprint(t, []int{8, 5, 5, 3}, 2, hours)
}

// get consulta las métricas del sprint con la sesión de Ana, exige un 200 y verifica el formato de
// todas las cifras de la respuesta.
func (f metricsFixture) get(t *testing.T, sprintID uuid.UUID) sprintMetricsBody {
	t.Helper()
	var body sprintMetricsBody
	rec := f.env.getSprintMetrics(t, f.ownerToken, f.projectID.String(), sprintID.String())
	metricsRequireStatus(t, rec, http.StatusOK, &body)
	requireWellFormed(t, body)
	return body
}

// requireWellFormed verifica que los Story Points y los conteos sean enteros, que horas y porcentajes
// tengan exactamente 2 decimales, que value sea null si y solo si la métrica no está disponible y que
// la lista de advertencias exista y esté vacía, porque los datos de US1 son consistentes.
func requireWellFormed(t *testing.T, body sprintMetricsBody) {
	t.Helper()
	check := func(name string, m metricValue, format *regexp.Regexp) {
		if m.Status == "available" {
			assert.Regexp(t, format, string(m.Value), "formato de %s", name)
			return
		}
		assert.Equal(t, "null", string(m.Value), "%s no disponible debe llevar value null", name)
	}
	check("plannedStoryPoints", body.PlannedStoryPoints, integerFormat)
	check("completedStoryPoints", body.CompletedStoryPoints, integerFormat)
	check("estimatedHours", body.EstimatedHours, decimalFormat)
	check("actualHours", body.ActualHours, decimalFormat)
	check("deviationHours", body.DeviationHours, decimalFormat)
	check("deviationPercent", body.DeviationPercent, decimalFormat)
	check("storyCompletionPercent", body.StoryCompletionPercent, decimalFormat)
	check("defectsDetected", body.DefectsDetected, integerFormat)
	check("defectsResolved", body.DefectsResolved, integerFormat)
	assert.JSONEq(t, "[]", string(body.Warnings), "warnings debe existir y estar vacía")
}

// requireValue exige que la métrica tenga el valor textual esperado, o el estado not_calculable con
// value null cuando se espera la constante notCalculable.
func requireValue(t *testing.T, name string, got metricValue, want string) {
	t.Helper()
	if want == notCalculable {
		assert.Equal(t, notCalculable, got.Status, "status de %s", name)
		assert.Equal(t, "null", string(got.Value), "value de %s", name)
		return
	}
	assert.Equal(t, "available", got.Status, "status de %s", name)
	assert.Equal(t, want, string(got.Value), "value de %s", name)
}

// requireFinality exige la marca de finalidad esperada de la métrica.
func requireFinality(t *testing.T, name string, got metricValue, want string) {
	t.Helper()
	if assert.NotNil(t, got.Finality, "finality de %s", name) {
		assert.Equal(t, want, *got.Finality, "finality de %s", name)
	}
}

// expectedValues agrupa los siete valores que la spec fija en cada escenario de US1.
type expectedValues struct {
	planned, completed, estimated, actual, deviationHours, deviationPercent, completion string
}

// requireValues compara las siete métricas de valor de la respuesta con las esperadas.
func requireValues(t *testing.T, body sprintMetricsBody, want expectedValues) {
	t.Helper()
	requireValue(t, "plannedStoryPoints", body.PlannedStoryPoints, want.planned)
	requireValue(t, "completedStoryPoints", body.CompletedStoryPoints, want.completed)
	requireValue(t, "estimatedHours", body.EstimatedHours, want.estimated)
	requireValue(t, "actualHours", body.ActualHours, want.actual)
	requireValue(t, "deviationHours", body.DeviationHours, want.deviationHours)
	requireValue(t, "deviationPercent", body.DeviationPercent, want.deviationPercent)
	requireValue(t, "storyCompletionPercent", body.StoryCompletionPercent, want.completion)
}

// requireFinalities exige las marcas de un sprint Cerrado: los Story Points, las horas estimadas y el
// porcentaje de historias, definitivos; las horas reales y las dos desviaciones, con la marca
// indicada según el plazo de gracia; y los dos conteos de defectos, siempre provisorios.
func requireFinalities(t *testing.T, body sprintMetricsBody, effortFinality string) {
	t.Helper()
	requireFinality(t, "plannedStoryPoints", body.PlannedStoryPoints, "definitive")
	requireFinality(t, "completedStoryPoints", body.CompletedStoryPoints, "definitive")
	requireFinality(t, "estimatedHours", body.EstimatedHours, "definitive")
	requireFinality(t, "storyCompletionPercent", body.StoryCompletionPercent, "definitive")
	requireFinality(t, "actualHours", body.ActualHours, effortFinality)
	requireFinality(t, "deviationHours", body.DeviationHours, effortFinality)
	requireFinality(t, "deviationPercent", body.DeviationPercent, effortFinality)
	requireFinality(t, "defectsDetected", body.DefectsDetected, "provisional")
	requireFinality(t, "defectsResolved", body.DefectsResolved, "provisional")
}

// normalValues son los valores de US1-1: 21, 13, 126, 142,5, +16,5, +13,10 % y 50,00 %.
var normalValues = expectedValues{
	planned: "21", completed: "13", estimated: "126.00", actual: "142.50",
	deviationHours: "16.50", deviationPercent: "13.10", completion: "50.00",
}

// Escenario: Caso normal — sprint Cerrado de 21 puntos planificados, 13 completados y 142,5 horas (008/US1)
func TestUS1_SprintCerradoCasoNormal(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.normalSprint(t, 142.5)

	body := f.get(t, sprintID)

	requireValues(t, body, normalValues)
	assert.Equal(t, sprintID.String(), body.Sprint.ID)
	assert.Equal(t, "Sprint 1", body.Sprint.Name)
	assert.Equal(t, "closed", body.Sprint.Status)
	assert.Equal(t, "2026-10-12", body.Sprint.StartDate)
	assert.Equal(t, "2026-10-23", body.Sprint.EndDate)
	if assert.NotNil(t, body.Sprint.ClosedAt, "closedAt de un sprint Cerrado") {
		closedAt, err := time.Parse(time.RFC3339, *body.Sprint.ClosedAt)
		require.NoError(t, err, "closedAt debe ser un instante RFC 3339")
		assert.True(t, closedAt.Equal(f.fridayClose()), "closedAt = %s, se esperaba %s", closedAt, f.fridayClose())
	}
	// Sin defectos registrados en esta historia, los dos conteos valen 0 como valor disponible.
	requireValue(t, "defectsDetected", body.DefectsDetected, "0")
	requireValue(t, "defectsResolved", body.DefectsResolved, "0")
}

// Escenario: Caso alternativo — 110 horas reales en lugar de 142,5 (sobrestimación) (008/US1)
func TestUS1_Sobrestimacion(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.normalSprint(t, 110)

	body := f.get(t, sprintID)

	requireValues(t, body, expectedValues{
		planned: "21", completed: "13", estimated: "126.00", actual: "110.00",
		deviationHours: "-16.00", deviationPercent: "-12.70", completion: "50.00",
	})
}

// Escenario: Caso alternativo — el propietario cambia el factor de 6 a 8 después del cierre (008/US1)
func TestUS1_CambioDeFactorPosterior(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.normalSprint(t, 142.5)

	f.env.changeFactor(t, f.ownerToken, f.projectID, metricsProjectName, 8)
	body := f.get(t, sprintID)

	// Las horas estimadas siguen siendo 21 × 6 y no 21 × 8 = 168, y la desviación no se mueve.
	requireValues(t, body, normalValues)
}

// Escenario: Caso alternativo — una historia del sprint se reestima de 5 a 13 después del cierre (008/US1)
func TestUS1_ReestimacionPosterior(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, stories := f.normalSprint(t, 142.5)

	// La tercera historia (5 puntos) no se completó, así que vuelve al backlog y admite reestimarse.
	f.env.estimateStory(t, f.ownerToken, stories[2], 13)
	body := f.get(t, sprintID)

	// Los puntos planificados salen de la instantánea del cierre, no de los puntos vigentes.
	requireValues(t, body, normalValues)
}

// Escenario: Caso alternativo — sprint Cerrado hace 10 horas con plazo de gracia de 48 horas (008/US1)
func TestUS1_EstadosDentroDelPlazoDeGracia(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.normalSprint(t, 142.5)

	f.env.clock.Advance(10 * time.Hour)
	body := f.get(t, sprintID)

	// Cuatro definitivas, tres provisorias y los dos conteos de defectos provisorios.
	requireValues(t, body, normalValues)
	requireFinalities(t, body, "provisional")
}

// Escenario: Caso alternativo — el mismo sprint una vez vencido el plazo de gracia (008/US1)
func TestUS1_EstadosConPlazoVencido(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.normalSprint(t, 142.5)

	f.env.clock.Advance(49 * time.Hour)
	body := f.get(t, sprintID)

	// Las horas reales y las dos desviaciones pasan a definitivas; los conteos siguen provisorios.
	requireValues(t, body, normalValues)
	requireFinalities(t, body, "definitive")
}

// Escenario: Caso alternativo — 3 horas más cargadas dentro del plazo de gracia (008/US1)
func TestUS1_EsfuerzoDentroDelPlazo(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, stories := f.normalSprint(t, 142.5)
	f.env.clock.Advance(10 * time.Hour)
	requireValues(t, f.get(t, sprintID), normalValues)

	f.env.logHours(t, f.ownerToken, stories[0], 3)
	body := f.get(t, sprintID)

	// Las horas reales son 145,5 y la desviación se recalcula: 19,5 y 19,5 / 126 × 100 = 15,48 %.
	requireValues(t, body, expectedValues{
		planned: "21", completed: "13", estimated: "126.00", actual: "145.50",
		deviationHours: "19.50", deviationPercent: "15.48", completion: "50.00",
	})
}

// Escenario: Caso límite — tres historias planificadas en 0 Story Points con 12 horas reales (008/US1)
func TestUS1_HistoriasEnCeroPuntos(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.closedSprint(t, []int{0, 0, 0}, 0, 12)

	body := f.get(t, sprintID)

	// Sin horas estimadas no hay base para el porcentaje: la desviación porcentual no es calculable.
	requireValues(t, body, expectedValues{
		planned: "0", completed: "0", estimated: "0.00", actual: "12.00",
		deviationHours: "12.00", deviationPercent: notCalculable, completion: "0.00",
	})
}

// Escenario: Caso límite — sprint Cerrado sin ninguna historia planificada (008/US1)
func TestUS1_SprintSinHistorias(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.closedSprint(t, nil, 0, 0)

	body := f.get(t, sprintID)

	// La desviación absoluta es igual a las horas reales, que son 0, y los dos cocientes no son
	// calculables.
	requireValues(t, body, expectedValues{
		planned: "0", completed: "0", estimated: "0.00", actual: "0.00",
		deviationHours: "0.00", deviationPercent: notCalculable, completion: notCalculable,
	})
}

// Escenario: Caso límite — cuatro historias planificadas y las cuatro completadas (008/US1)
func TestUS1_TodasCompletadas(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.closedSprint(t, []int{5, 3, 2, 1}, 4, 0)

	body := f.get(t, sprintID)

	requireValue(t, "storyCompletionPercent", body.StoryCompletionPercent, "100.00")
	requireValue(t, "plannedStoryPoints", body.PlannedStoryPoints, "11")
	requireValue(t, "completedStoryPoints", body.CompletedStoryPoints, "11")
}

// Escenario: Caso límite — tres historias planificadas y una completada (008/US1)
func TestUS1_DecimalPeriodico(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.closedSprint(t, []int{5, 3, 2}, 1, 0)

	body := f.get(t, sprintID)

	// 1 / 3 × 100 = 33,333…, redondeado solo en el resultado final; los puntos no intervienen.
	requireValue(t, "storyCompletionPercent", body.StoryCompletionPercent, "33.33")
	requireValue(t, "plannedStoryPoints", body.PlannedStoryPoints, "10")
	requireValue(t, "completedStoryPoints", body.CompletedStoryPoints, "5")
}

// Escenario: Caso límite — historia de 5 puntos replanificada del sprint 1 al sprint 2 (008/US1)
func TestUS1_HistoriaReplanificada(t *testing.T) {
	f := newMetricsFixture(t)
	storyID := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", 5)
	first := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-12", "2026-10-23", storyID)
	f.env.startSprint(t, f.ownerToken, first)
	f.env.closeSprintAt(t, f.ownerToken, first, f.fridayClose())
	second := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 2", "2026-10-26", "2026-11-06", storyID)
	f.env.startSprint(t, f.ownerToken, second)
	f.env.completeStory(t, f.ownerToken, storyID)
	f.env.closeSprintAt(t, f.ownerToken, second, f.env.at(2026, time.November, 6, 18, 0))

	firstBody := f.get(t, first)
	secondBody := f.get(t, second)

	// Planificada en los dos sprints, completada solo en el segundo.
	requireValue(t, "sprint 1 plannedStoryPoints", firstBody.PlannedStoryPoints, "5")
	requireValue(t, "sprint 1 completedStoryPoints", firstBody.CompletedStoryPoints, "0")
	requireValue(t, "sprint 1 storyCompletionPercent", firstBody.StoryCompletionPercent, "0.00")
	requireValue(t, "sprint 2 plannedStoryPoints", secondBody.PlannedStoryPoints, "5")
	requireValue(t, "sprint 2 completedStoryPoints", secondBody.CompletedStoryPoints, "5")
	requireValue(t, "sprint 2 storyCompletionPercent", secondBody.StoryCompletionPercent, "100.00")
}

// Escenario: Caso de error — identificador de sprint que no existe (008/US1)
func TestUS1_SprintInexistente(t *testing.T) {
	f := newMetricsFixture(t)
	bodies := map[string]string{}

	for name, sprintID := range map[string]string{
		"UUID inexistente":         uuid.NewString(),
		"identificador no es UUID": "abc",
	} {
		t.Run(name, func(t *testing.T) {
			rec := f.env.getSprintMetrics(t, f.ownerToken, f.projectID.String(), sprintID)

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, sprintNotFoundBody, rec.Body.String())
			bodies[name] = rec.Body.String()
		})
	}

	// Los dos casos son indistinguibles: el mismo cuerpo byte a byte.
	require.Len(t, bodies, 2)
	assert.Equal(t, bodies["UUID inexistente"], bodies["identificador no es UUID"])
}

// Escenario: Caso de error — sprint de otro proyecto (008/US1)
func TestUS1_SprintDeOtroProyecto(t *testing.T) {
	f := newMetricsFixture(t)
	_, brunoToken := f.env.signUp(t, "Bruno Díaz", "bruno@example.com")
	other := metricsFixture{env: f.env, ownerToken: brunoToken, projectID: f.env.createProject(t, brunoToken, "Otro proyecto", 6)}
	foreignSprint, _ := other.normalSprint(t, 142.5)

	foreign := f.env.getSprintMetrics(t, f.ownerToken, f.projectID.String(), foreignSprint.String())
	missing := f.env.getSprintMetrics(t, f.ownerToken, f.projectID.String(), uuid.NewString())

	// La respuesta es idéntica byte a byte a la de un sprint inexistente y no revela datos ajenos.
	assert.Equal(t, http.StatusNotFound, foreign.Code)
	assert.Equal(t, sprintNotFoundBody, foreign.Body.String())
	assert.Equal(t, missing.Body.String(), foreign.Body.String())
	assert.NotContains(t, foreign.Body.String(), foreignSprint.String())
}

// Escenario: Caso de error — usuario que no es integrante del proyecto (008/US1)
func TestUS1_NoIntegrante(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.normalSprint(t, 142.5)
	_, brunoToken := f.env.signUp(t, "Bruno Díaz", "bruno@example.com")

	rec := f.env.getSprintMetrics(t, brunoToken, f.projectID.String(), sprintID.String())

	// Quien no es integrante recibe la respuesta de proyecto inexistente, sin ningún dato del sprint.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, projectNotFoundBody, rec.Body.String())
}

// ---------------------------------------------------------------------------------------------
// US2: velocidad del equipo y series del proyecto (008/US2)
// ---------------------------------------------------------------------------------------------

// velocityPoint es un punto de velocity.series: los Story Points completados de un sprint Cerrado.
type velocityPoint struct {
	SprintID             string  `json:"sprintId"`
	SprintName           string  `json:"sprintName"`
	ClosedAt             *string `json:"closedAt"`
	CompletedStoryPoints int     `json:"completedStoryPoints"`
}

// projectMetricsBody es la parte de GET /metrics que cubre US2: la velocidad (contracts/openapi.yaml).
type projectMetricsBody struct {
	ProjectID string `json:"projectId"`
	Velocity  struct {
		Value             metricValue     `json:"value"`
		SprintsConsidered int             `json:"sprintsConsidered"`
		Series            []velocityPoint `json:"series"`
	} `json:"velocity"`
}

// seriesPoint es un punto de las series: plano, con el sprint al que corresponde.
type seriesPoint struct {
	SprintID     string          `json:"sprintId"`
	SprintName   string          `json:"sprintName"`
	SprintStatus string          `json:"sprintStatus"`
	Status       string          `json:"status"`
	Value        json.RawMessage `json:"value"`
	Finality     *string         `json:"finality"`
}

// projectSeriesBody es la respuesta de GET /metrics/series: las seis listas de puntos.
type projectSeriesBody struct {
	PlannedStoryPoints   []seriesPoint `json:"plannedStoryPoints"`
	CompletedStoryPoints []seriesPoint `json:"completedStoryPoints"`
	EstimatedHours       []seriesPoint `json:"estimatedHours"`
	ActualHours          []seriesPoint `json:"actualHours"`
	DefectsDetected      []seriesPoint `json:"defectsDetected"`
	DefectsResolved      []seriesPoint `json:"defectsResolved"`
}

// lists devuelve las seis series con su nombre, para recorrerlas juntas.
func (b projectSeriesBody) lists() map[string][]seriesPoint {
	return map[string][]seriesPoint{
		"plannedStoryPoints":   b.PlannedStoryPoints,
		"completedStoryPoints": b.CompletedStoryPoints,
		"estimatedHours":       b.EstimatedHours,
		"actualHours":          b.ActualHours,
		"defectsDetected":      b.DefectsDetected,
		"defectsResolved":      b.DefectsResolved,
	}
}

// getProjectMetrics envía GET /api/v1/projects/{projectId}/metrics con el token.
func (e *metricsTestEnv) getProjectMetrics(t *testing.T, token, projectID string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, "/api/v1/projects/"+projectID+"/metrics", token, nil)
}

// getProjectSeries envía GET /api/v1/projects/{projectId}/metrics/series con el token.
func (e *metricsTestEnv) getProjectSeries(t *testing.T, token, projectID string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, "/api/v1/projects/"+projectID+"/metrics/series", token, nil)
}

// startedSprintN arma el sprint número n (desde 0) del proyecto, de dos semanas y sin solaparse con
// los anteriores: una historia por cada valor de done (completadas) y de notDone (sin completar),
// lo inicia, completa las primeras y carga hours horas sobre la primera historia. Devuelve el sprint
// y su cierre real, el viernes de su último día a las 18:00.
func (f metricsFixture) startedSprintN(t *testing.T, n int, done, notDone []int, hours float64) (uuid.UUID, time.Time) {
	t.Helper()
	start := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 14*n)
	end := start.AddDate(0, 0, 11)
	points := append(append([]int{}, done...), notDone...)
	stories := make([]uuid.UUID, 0, len(points))
	for i, p := range points {
		stories = append(stories, f.env.createStory(t, f.ownerToken, f.projectID, "Historia "+strconv.Itoa(n)+string(rune('A'+i)), p))
	}
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint "+strconv.Itoa(n+1),
		start.Format(time.DateOnly), end.Format(time.DateOnly), stories...)
	f.env.startSprint(t, f.ownerToken, sprintID)
	for _, storyID := range stories[:len(done)] {
		f.env.completeStory(t, f.ownerToken, storyID)
	}
	if hours > 0 {
		f.env.logHours(t, f.ownerToken, stories[0], hours)
	}
	return sprintID, f.env.at(end.Year(), end.Month(), end.Day(), 18, 0)
}

// closedSprintN arma y cierra el sprint número n con startedSprintN.
func (f metricsFixture) closedSprintN(t *testing.T, n int, done, notDone []int, hours float64) uuid.UUID {
	t.Helper()
	sprintID, closeAt := f.startedSprintN(t, n, done, notDone, hours)
	f.env.closeSprintAt(t, f.ownerToken, sprintID, closeAt)
	return sprintID
}

// velocitySprints cierra en orden un sprint por cada valor de completados, descompuesto en historias
// de la escala de Story Points (13, 21 = 13 + 8, 8 y 20), y devuelve sus identificadores.
func (f metricsFixture) velocitySprints(t *testing.T, completed ...int) []uuid.UUID {
	t.Helper()
	split := map[int][]int{0: nil, 8: {8}, 13: {13}, 20: {20}, 21: {13, 8}}
	ids := make([]uuid.UUID, 0, len(completed))
	for n, c := range completed {
		parts, ok := split[c]
		require.True(t, ok, "valor de velocidad %d sin descomposición de prueba", c)
		// Un sprint sin nada completado necesita al menos una historia sin completar.
		var notDone []int
		if c == 0 {
			notDone = []int{3}
		}
		ids = append(ids, f.closedSprintN(t, n, parts, notDone, 0))
	}
	return ids
}

// projectMetrics consulta GET /metrics con la sesión de Ana y exige un 200.
func (f metricsFixture) projectMetrics(t *testing.T) projectMetricsBody {
	t.Helper()
	var body projectMetricsBody
	metricsRequireStatus(t, f.env.getProjectMetrics(t, f.ownerToken, f.projectID.String()), http.StatusOK, &body)
	return body
}

// projectSeries consulta GET /metrics/series con la sesión de Ana y exige un 200.
func (f metricsFixture) projectSeries(t *testing.T) projectSeriesBody {
	t.Helper()
	var body projectSeriesBody
	metricsRequireStatus(t, f.env.getProjectSeries(t, f.ownerToken, f.projectID.String()), http.StatusOK, &body)
	return body
}

// seriesValues devuelve el texto del value de cada punto, en el orden de la serie.
func seriesValues(points []seriesPoint) []string {
	values := make([]string, 0, len(points))
	for _, p := range points {
		values = append(values, string(p.Value))
	}
	return values
}

// requireVelocity exige el valor, el estado y la cantidad de sprints considerados de la velocidad.
func requireVelocity(t *testing.T, body projectMetricsBody, want string, considered int) {
	t.Helper()
	requireValue(t, "velocity", body.Velocity.Value, want)
	assert.Equal(t, considered, body.Velocity.SprintsConsidered, "sprintsConsidered")
}

// Escenario: Caso normal — tres sprints cerrados que completaron 13, 21 y 8 Story Points (008/US2)
func TestUS2_VelocidadTresCerrados(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13, 21, 8)

	body := f.projectMetrics(t)

	// (13 + 21 + 8) / 3 = 14,00.
	requireVelocity(t, body, "14.00", 3)
}

// Escenario: Caso normal — cuatro sprints cerrados que completaron 13, 21, 8 y 20 Story Points (008/US2)
func TestUS2_VentanaDeTres(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13, 21, 8, 20)

	body := f.projectMetrics(t)

	// Solo cuentan los 3 últimos: (21 + 8 + 20) / 3 = 16,33 y el primero queda fuera (SC-004).
	requireVelocity(t, body, "16.33", 3)
}

// Escenario: Caso alternativo — exactamente un sprint cerrado que completó 13 Story Points (008/US2)
func TestUS2_UnSprintCerrado(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13)

	body := f.projectMetrics(t)

	requireVelocity(t, body, "13.00", 1)
}

// Escenario: Caso alternativo — exactamente dos sprints cerrados que completaron 13 y 21 Story Points (008/US2)
func TestUS2_DosSprintsCerrados(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13, 21)

	body := f.projectMetrics(t)

	// (13 + 21) / 2 = 17,00.
	requireVelocity(t, body, "17.00", 2)
}

// Escenario: Caso alternativo — la serie de velocidad de cuatro sprints cerrados (008/US2)
func TestUS2_SerieDeVelocidad(t *testing.T) {
	f := newMetricsFixture(t)
	ids := f.velocitySprints(t, 13, 21, 8, 20)

	body := f.projectMetrics(t)

	// Los cuatro valores en orden cronológico, sin promediar y sin recortar a los últimos 3.
	require.Len(t, body.Velocity.Series, 4)
	for i, want := range []int{13, 21, 8, 20} {
		point := body.Velocity.Series[i]
		assert.Equal(t, want, point.CompletedStoryPoints, "completedStoryPoints del punto %d", i)
		assert.Equal(t, ids[i].String(), point.SprintID, "sprintId del punto %d", i)
		assert.Equal(t, "Sprint "+strconv.Itoa(i+1), point.SprintName, "sprintName del punto %d", i)
		assert.NotNil(t, point.ClosedAt, "closedAt del punto %d", i)
	}
}

// Escenario: Caso normal — las seis series del proyecto en una sola respuesta (008/US2)
func TestUS2_SeisSeries(t *testing.T) {
	f := newMetricsFixture(t)
	// Cuatro cerrados con 13, 21, 8 y 20 completados más una historia de 3 sin completar, y 10 horas
	// reales en el segundo.
	ids := make([]uuid.UUID, 0, 4)
	for n, done := range [][]int{{13}, {13, 8}, {8}, {20}} {
		hours := 0.0
		if n == 1 {
			hours = 10
		}
		ids = append(ids, f.closedSprintN(t, n, done, []int{3}, hours))
	}

	body := f.projectSeries(t)

	want := map[string][]string{
		"plannedStoryPoints":   {"16", "24", "11", "23"},
		"completedStoryPoints": {"13", "21", "8", "20"},
		"estimatedHours":       {"96.00", "144.00", "66.00", "138.00"},
		"actualHours":          {"0.00", "10.00", "0.00", "0.00"},
		"defectsDetected":      {"0", "0", "0", "0"},
		"defectsResolved":      {"0", "0", "0", "0"},
	}
	for name, points := range body.lists() {
		require.Len(t, points, 4, "puntos de %s", name)
		assert.Equal(t, want[name], seriesValues(points), "valores de %s", name)
		for i, point := range points {
			assert.Equal(t, ids[i].String(), point.SprintID, "sprintId de %s[%d]", name, i)
			assert.Equal(t, "Sprint "+strconv.Itoa(i+1), point.SprintName, "sprintName de %s[%d]", name, i)
			assert.Equal(t, "closed", point.SprintStatus, "sprintStatus de %s[%d]", name, i)
			assert.Equal(t, "available", point.Status, "status de %s[%d]", name, i)
		}
	}
}

// Escenario: Caso alternativo — tres sprints cerrados y uno Activo (008/US2)
func TestUS2_SeriesConSprintActivo(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13, 21, 8)
	activeID, _ := f.startedSprintN(t, 3, []int{5}, []int{3}, 4)

	body := f.projectSeries(t)

	// Las seis incluyen los 4 sprints y los puntos del Activo vienen marcados como parciales.
	for name, points := range body.lists() {
		require.Len(t, points, 4, "puntos de %s", name)
		assert.Equal(t, activeID.String(), points[3].SprintID, "sprintId del último punto de %s", name)
		assert.Equal(t, "active", points[3].SprintStatus, "sprintStatus del Activo en %s", name)
		if assert.NotNil(t, points[3].Finality, "finality del Activo en %s", name) {
			assert.Equal(t, "partial", *points[3].Finality, "finality del Activo en %s", name)
		}
		assert.Equal(t, "closed", points[0].SprintStatus, "sprintStatus del primero en %s", name)
	}
	assert.Equal(t, []string{"13", "21", "8", "5"}, seriesValues(body.CompletedStoryPoints))
}

// Escenario: Caso alternativo — un sprint Planificado además de los cerrados (008/US2)
func TestUS2_SeriesSinPlanificados(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13, 21)
	storyID := f.env.createStory(t, f.ownerToken, f.projectID, "Historia futura", 5)
	plannedID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 3", "2026-11-09", "2026-11-20", storyID)

	body := f.projectSeries(t)

	// El Planificado no aparece en ninguna de las seis.
	for name, points := range body.lists() {
		require.Len(t, points, 2, "puntos de %s", name)
		for _, point := range points {
			assert.NotEqual(t, plannedID.String(), point.SprintID, "el Planificado aparece en %s", name)
		}
	}
}

// Escenario: Caso límite — un sprint sin Story Points completados ni horas registradas (008/US2)
func TestUS2_ValorCeroEnSeries(t *testing.T) {
	f := newMetricsFixture(t)
	f.closedSprintN(t, 0, nil, []int{5}, 0)

	body := f.projectSeries(t)

	// El sprint aparece con valor 0 disponible, distinguible de la ausencia de dato.
	for name, want := range map[string]string{
		"completedStoryPoints": "0",
		"actualHours":          "0.00",
		"defectsDetected":      "0",
		"defectsResolved":      "0",
	} {
		points := body.lists()[name]
		require.Len(t, points, 1, "puntos de %s", name)
		assert.Equal(t, "available", points[0].Status, "status de %s", name)
		assert.Equal(t, want, string(points[0].Value), "value de %s", name)
	}
}

// Escenario: Caso límite — proyecto sin sprints cerrados ni activo (008/US2)
func TestUS2_SeriesVacias(t *testing.T) {
	f := newMetricsFixture(t)

	body := f.projectSeries(t)

	// Las seis vienen como lista vacía y no como null, sin que sea un error.
	for name, points := range body.lists() {
		assert.NotNil(t, points, "%s debe ser [] y no null", name)
		assert.Empty(t, points, "%s debe estar vacía", name)
	}
}

// Escenario: Caso límite — tres sprints cerrados que no completaron ninguna historia (008/US2)
func TestUS2_VelocidadCero(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 0, 0, 0)

	body := f.projectMetrics(t)

	// 0,00 es un promedio válido y distinto de "no calculable".
	requireVelocity(t, body, "0.00", 3)
}

// Escenario: Caso límite — ningún sprint cerrado y un sprint Activo (008/US2)
func TestUS2_SoloSprintActivo(t *testing.T) {
	f := newMetricsFixture(t)
	f.startedSprintN(t, 0, []int{5}, []int{3}, 4)

	body := f.projectMetrics(t)

	// La velocidad solo mira sprints cerrados: no calculable y serie vacía.
	requireVelocity(t, body, notCalculable, 0)
	assert.NotNil(t, body.Velocity.Series, "velocity.series debe ser [] y no null")
	assert.Empty(t, body.Velocity.Series)
}

// Escenario: Caso límite — proyecto sin ningún sprint (008/US2)
func TestUS2_ProyectoSinSprints(t *testing.T) {
	f := newMetricsFixture(t)

	body := f.projectMetrics(t)

	// Responde 200 y no es un error: velocidad no calculable y serie vacía.
	requireVelocity(t, body, notCalculable, 0)
	assert.NotNil(t, body.Velocity.Series, "velocity.series debe ser [] y no null")
	assert.Empty(t, body.Velocity.Series)
}

// Escenario: Caso límite — tres sprints cerrados y se cierra un cuarto (008/US2)
func TestUS2_VentanaQueSeDesplaza(t *testing.T) {
	f := newMetricsFixture(t)
	ids := f.velocitySprints(t, 13, 21, 8)
	requireVelocity(t, f.projectMetrics(t), "14.00", 3)
	firstBefore := f.get(t, ids[0])

	f.closedSprintN(t, 3, []int{20}, nil, 0)
	body := f.projectMetrics(t)

	// El valor cambia porque la ventana se desplaza y el sprint que sale conserva sus métricas.
	requireVelocity(t, body, "16.33", 3)
	assert.Equal(t, firstBefore, f.get(t, ids[0]), "el primer sprint debe conservar sus métricas")
}

// Escenario: Caso de error — usuario que no es integrante del proyecto (008/US2)
func TestUS2_NoIntegrante(t *testing.T) {
	f := newMetricsFixture(t)
	f.velocitySprints(t, 13)
	_, brunoToken := f.env.signUp(t, "Bruno Díaz", "bruno@example.com")

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"velocidad": f.env.getProjectMetrics(t, brunoToken, f.projectID.String()),
		"series":    f.env.getProjectSeries(t, brunoToken, f.projectID.String()),
	} {
		t.Run(name, func(t *testing.T) {
			// Quien no es integrante recibe la respuesta de proyecto inexistente, sin ningún dato.
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, projectNotFoundBody, rec.Body.String())
		})
	}
}

// ---------------------------------------------------------------------------------------------
// US3: métricas acumuladas del proyecto (008/US3)
// ---------------------------------------------------------------------------------------------

// cumulativeBlock es el bloque cumulative de GET /metrics: la suma de los sprints Cerrados y del
// Activo, con una sola marca de finalidad para todo el bloque (contracts/openapi.yaml, Cumulative).
type cumulativeBlock struct {
	Finality               string      `json:"finality"`
	IncludesActiveSprint   bool        `json:"includesActiveSprint"`
	PlannedStoryPoints     metricValue `json:"plannedStoryPoints"`
	CompletedStoryPoints   metricValue `json:"completedStoryPoints"`
	EstimatedHours         metricValue `json:"estimatedHours"`
	ActualHours            metricValue `json:"actualHours"`
	DeviationHours         metricValue `json:"deviationHours"`
	DeviationPercent       metricValue `json:"deviationPercent"`
	StoryCompletionPercent metricValue `json:"storyCompletionPercent"`
}

// projectWarning es una advertencia de la respuesta del proyecto.
type projectWarning struct {
	Kind  string `json:"kind"`
	Code  string `json:"code"`
	Count *int   `json:"count"`
}

// cumulativeBody es la parte de GET /metrics que cubre US3. Las tres cifras sueltas quedan como
// texto JSON crudo: si el campo falta, el texto queda vacío y la prueba falla, en vez de leerse 0.
type cumulativeBody struct {
	Velocity struct {
		Value metricValue `json:"value"`
	} `json:"velocity"`
	Cumulative                  cumulativeBlock  `json:"cumulative"`
	EffortOutsideSprintsHours   json.RawMessage  `json:"effortOutsideSprintsHours"`
	EffortInPlannedSprintsHours json.RawMessage  `json:"effortInPlannedSprintsHours"`
	UnestimatedBacklogStories   json.RawMessage  `json:"unestimatedBacklogStories"`
	Warnings                    []projectWarning `json:"warnings"`
}

// cumulative consulta GET /metrics con la sesión de Ana, exige un 200 y verifica el formato de las
// cifras del bloque acumulado.
func (f metricsFixture) cumulative(t *testing.T) cumulativeBody {
	t.Helper()
	var body cumulativeBody
	metricsRequireStatus(t, f.env.getProjectMetrics(t, f.ownerToken, f.projectID.String()), http.StatusOK, &body)
	c := body.Cumulative
	for name, check := range map[string]struct {
		m      metricValue
		format *regexp.Regexp
	}{
		"plannedStoryPoints":     {c.PlannedStoryPoints, integerFormat},
		"completedStoryPoints":   {c.CompletedStoryPoints, integerFormat},
		"estimatedHours":         {c.EstimatedHours, decimalFormat},
		"actualHours":            {c.ActualHours, decimalFormat},
		"deviationHours":         {c.DeviationHours, decimalFormat},
		"deviationPercent":       {c.DeviationPercent, decimalFormat},
		"storyCompletionPercent": {c.StoryCompletionPercent, decimalFormat},
	} {
		if check.m.Status == "available" {
			assert.Regexp(t, check.format, string(check.m.Value), "formato de cumulative.%s", name)
		} else {
			assert.Equal(t, "null", string(check.m.Value), "cumulative.%s no disponible debe llevar value null", name)
		}
	}
	return body
}

// requireCumulative compara las siete métricas del bloque acumulado con las esperadas.
func requireCumulative(t *testing.T, c cumulativeBlock, want expectedValues) {
	t.Helper()
	requireValue(t, "cumulative.plannedStoryPoints", c.PlannedStoryPoints, want.planned)
	requireValue(t, "cumulative.completedStoryPoints", c.CompletedStoryPoints, want.completed)
	requireValue(t, "cumulative.estimatedHours", c.EstimatedHours, want.estimated)
	requireValue(t, "cumulative.actualHours", c.ActualHours, want.actual)
	requireValue(t, "cumulative.deviationHours", c.DeviationHours, want.deviationHours)
	requireValue(t, "cumulative.deviationPercent", c.DeviationPercent, want.deviationPercent)
	requireValue(t, "cumulative.storyCompletionPercent", c.StoryCompletionPercent, want.completion)
}

// requireEffortFigures exige las tres cifras sueltas de la respuesta del proyecto.
func requireEffortFigures(t *testing.T, body cumulativeBody, outside, inPlanned, unestimated string) {
	t.Helper()
	assert.Equal(t, outside, string(body.EffortOutsideSprintsHours), "effortOutsideSprintsHours")
	assert.Equal(t, inPlanned, string(body.EffortInPlannedSprintsHours), "effortInPlannedSprintsHours")
	assert.Equal(t, unestimated, string(body.UnestimatedBacklogStories), "unestimatedBacklogStories")
}

// hasWarning indica si la lista trae una advertencia con ese código.
func hasWarning(warnings []projectWarning, code string) bool {
	for _, w := range warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

// accumulatedValues son los valores de US3-1: 63, 39, 378, 312,5, -65,5, -17,33 % y 63,64 %.
var accumulatedValues = expectedValues{
	planned: "63", completed: "39", estimated: "378.00", actual: "312.50",
	deviationHours: "-65.50", deviationPercent: "-17.33", completion: "63.64",
}

// accumulatedProject arma el "Dado" de US3: dos sprints Cerrados (21 planificados y 13 completados
// con 142,5 horas; 24 y 21 con 130 horas) y uno Activo (18 y 5 con 40 horas), con 11 historias
// planificadas y 7 completadas en total. Devuelve el sprint Activo y su cierre real.
func (f metricsFixture) accumulatedProject(t *testing.T) (uuid.UUID, time.Time) {
	t.Helper()
	f.closedSprintN(t, 0, []int{5, 5, 3}, []int{8}, 142.5)
	f.closedSprintN(t, 1, []int{8, 8, 5}, []int{3}, 130)
	return f.startedSprintN(t, 2, []int{5}, []int{8, 5}, 40)
}

// plannedSprintWith crea un sprint Planificado de diciembre con una historia de 5 puntos y, si
// hours es mayor que 0, le carga esas horas. Devuelve la historia.
func (f metricsFixture) plannedSprintWith(t *testing.T, hours float64) uuid.UUID {
	t.Helper()
	storyID := f.env.createStory(t, f.ownerToken, f.projectID, "Historia futura", 5)
	f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint futuro", "2026-12-07", "2026-12-18", storyID)
	if hours > 0 {
		f.env.logHours(t, f.ownerToken, storyID, hours)
	}
	return storyID
}

// createUnestimatedStory crea en el backlog una historia sin Story Points.
func (f metricsFixture) createUnestimatedStory(t *testing.T, title string) {
	t.Helper()
	metricsRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/projects/"+f.projectID.String()+"/stories", f.ownerToken, map[string]any{
		"title":              title,
		"priority":           "high",
		"storyPoints":        nil,
		"acceptanceCriteria": []string{metricsCriterion},
	}), http.StatusCreated, nil)
}

// Escenario: Caso normal — dos sprints cerrados y uno Activo suman 63, 39, 378, 312,5 y desviaciones (008/US3)
func TestUS3_AcumuladosCasoNormal(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)

	body := f.cumulative(t)

	// -65,5 / 378 x 100 = -17,3280... y se redondea a -17,33.
	requireCumulative(t, body.Cumulative, accumulatedValues)
}

// Escenario: Caso alternativo — 11 historias planificadas y 7 completadas en total (008/US3)
func TestUS3_PorcentajeAcumulado(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)

	body := f.cumulative(t)

	// 7 / 11 x 100 = 63,6363... sale de los totales y no del promedio de los porcentajes por sprint.
	requireValue(t, "cumulative.storyCompletionPercent", body.Cumulative.StoryCompletionPercent, "63.64")
}

// Escenario: Caso alternativo — un sprint Planificado además de los anteriores (008/US3)
func TestUS3_SinPlanificados(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)
	f.plannedSprintWith(t, 0)

	body := f.cumulative(t)

	// El Planificado no aporta nada: los acumulados son los mismos.
	requireCumulative(t, body.Cumulative, accumulatedValues)
}

// Escenario: Caso alternativo — el proyecto tiene un sprint Activo (008/US3)
func TestUS3_AcumuladoParcial(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)

	body := f.cumulative(t)

	// El bloque es parcial porque incluye un sprint a mitad de camino, y la velocidad (17,00) no
	// lleva marca porque solo mira sprints cerrados.
	assert.Equal(t, "partial", body.Cumulative.Finality, "cumulative.finality")
	assert.True(t, body.Cumulative.IncludesActiveSprint, "cumulative.includesActiveSprint")
	requireValue(t, "velocity", body.Velocity.Value, "17.00")
	assert.Nil(t, body.Velocity.Value.Finality, "velocity no debe llevar finality")
}

// Escenario: Caso alternativo — el sprint activo se cierra y no se inicia ninguno nuevo (008/US3)
func TestUS3_AcumuladoDefinitivo(t *testing.T) {
	f := newMetricsFixture(t)
	activeID, closeAt := f.accumulatedProject(t)
	f.env.closeSprintAt(t, f.ownerToken, activeID, closeAt)

	body := f.cumulative(t)

	// Sin sprint en curso el conjunto pasa a definitivo (SC-018) y los totales no cambian.
	assert.Equal(t, "definitive", body.Cumulative.Finality, "cumulative.finality")
	assert.False(t, body.Cumulative.IncludesActiveSprint, "cumulative.includesActiveSprint")
	requireCumulative(t, body.Cumulative, accumulatedValues)
}

// Escenario: Caso alternativo — historia de 5 puntos planificada en dos sprints y completada en el segundo (008/US3)
func TestUS3_HistoriaEnDosSprints(t *testing.T) {
	f := newMetricsFixture(t)
	storyID := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", 5)
	first := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-12", "2026-10-23", storyID)
	f.env.startSprint(t, f.ownerToken, first)
	f.env.closeSprintAt(t, f.ownerToken, first, f.fridayClose())
	second := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 2", "2026-10-26", "2026-11-06", storyID)
	f.env.startSprint(t, f.ownerToken, second)
	f.env.completeStory(t, f.ownerToken, storyID)
	f.env.closeSprintAt(t, f.ownerToken, second, f.env.at(2026, time.November, 6, 18, 0))

	body := f.cumulative(t)

	// Se cuenta una vez por cada sprint en que estuvo planificada.
	requireValue(t, "cumulative.plannedStoryPoints", body.Cumulative.PlannedStoryPoints, "10")
	requireValue(t, "cumulative.completedStoryPoints", body.Cumulative.CompletedStoryPoints, "5")
}

// Escenario: Caso límite — los únicos sprints del proyecto están Planificados (008/US3)
func TestUS3_SoloSprintsPlanificados(t *testing.T) {
	f := newMetricsFixture(t)
	f.plannedSprintWith(t, 0)

	body := f.cumulative(t)

	// Totales en 0; los cocientes con denominador cero no son calculables.
	requireCumulative(t, body.Cumulative, expectedValues{
		planned: "0", completed: "0", estimated: "0.00", actual: "0.00",
		deviationHours: "0.00", deviationPercent: notCalculable, completion: notCalculable,
	})
	assert.False(t, body.Cumulative.IncludesActiveSprint, "cumulative.includesActiveSprint")
}

// Escenario: Caso límite — proyecto sin ningún sprint (008/US3)
func TestUS3_ProyectoSinSprints(t *testing.T) {
	f := newMetricsFixture(t)

	body := f.cumulative(t)

	// Responde 200 con totales en 0 y desviaciones no calculables, sin que sea un error.
	requireCumulative(t, body.Cumulative, expectedValues{
		planned: "0", completed: "0", estimated: "0.00", actual: "0.00",
		deviationHours: "0.00", deviationPercent: notCalculable, completion: notCalculable,
	})
}

// Escenario: Caso límite — sprints cerrados con 0 horas estimadas y horas reales registradas (008/US3)
func TestUS3_EstimadasEnCero(t *testing.T) {
	f := newMetricsFixture(t)
	f.closedSprint(t, []int{0, 0, 0}, 0, 12)

	body := f.cumulative(t)

	// La desviación absoluta es igual a las reales y la porcentual no es calculable.
	requireCumulative(t, body.Cumulative, expectedValues{
		planned: "0", completed: "0", estimated: "0.00", actual: "12.00",
		deviationHours: "12.00", deviationPercent: notCalculable, completion: "0.00",
	})
}

// Escenario: Caso alternativo — 18 horas registradas sobre historias solo del backlog (008/US3)
func TestUS3_EsfuerzoFueraDeSprint(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)
	backlogStory := f.env.createStory(t, f.ownerToken, f.projectID, "Historia del backlog", 5)
	f.env.logHours(t, f.ownerToken, backlogStory, 18)

	body := f.cumulative(t)

	// Las 18 horas van aparte y las reales acumuladas siguen cuadrando con la suma de los sprints.
	requireValue(t, "cumulative.actualHours", body.Cumulative.ActualHours, "312.50")
	assert.Equal(t, "18.00", string(body.EffortOutsideSprintsHours), "effortOutsideSprintsHours")
}

// Escenario: Caso límite — todo el esfuerzo se cargó dentro de sprints (008/US3)
func TestUS3_EsfuerzoFueraDeSprintEnCero(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)

	body := f.cumulative(t)

	// La cifra se informa como 0.00 y no se omite.
	assert.Equal(t, "0.00", string(body.EffortOutsideSprintsHours), "effortOutsideSprintsHours")
}

// Escenario: Caso alternativo — 9 horas registradas sobre historias de un sprint Planificado (008/US3)
func TestUS3_EsfuerzoEnSprintsPlanificados(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)

	t.Run("sin horas en Planificados", func(t *testing.T) {
		body := f.cumulative(t)

		// Si no hay horas en sprints Planificados la cifra se informa como 0.00 y no se omite.
		assert.Equal(t, "0.00", string(body.EffortInPlannedSprintsHours), "effortInPlannedSprintsHours")
	})

	t.Run("con 9 horas en un Planificado", func(t *testing.T) {
		f.plannedSprintWith(t, 9)
		backlogStory := f.env.createStory(t, f.ownerToken, f.projectID, "Historia del backlog", 5)
		f.env.logHours(t, f.ownerToken, backlogStory, 18)

		body := f.cumulative(t)

		// Las reales siguen en 312,5 y las tres cifras suman el total de horas registradas (SC-016):
		// 312,5 + 18 + 9 = 339,5.
		requireValue(t, "cumulative.actualHours", body.Cumulative.ActualHours, "312.50")
		assert.Equal(t, "9.00", string(body.EffortInPlannedSprintsHours), "effortInPlannedSprintsHours")
		assert.Equal(t, "18.00", string(body.EffortOutsideSprintsHours), "effortOutsideSprintsHours")
		actual, err := strconv.ParseFloat(string(body.Cumulative.ActualHours.Value), 64)
		require.NoError(t, err)
		outside, err := strconv.ParseFloat(string(body.EffortOutsideSprintsHours), 64)
		require.NoError(t, err)
		planned, err := strconv.ParseFloat(string(body.EffortInPlannedSprintsHours), 64)
		require.NoError(t, err)
		assert.InDelta(t, 339.5, actual+outside+planned, 0.001, "suma de las tres cifras")
	})
}

// Escenario: Caso alternativo — el backlog tiene 6 historias sin estimar (008/US3)
func TestUS3_BacklogSinEstimar(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)
	for i := range 6 {
		f.createUnestimatedStory(t, "Sin estimar "+strconv.Itoa(i))
	}

	body := f.cumulative(t)

	// El contador es 6 y viene la advertencia de planificación con la cantidad.
	assert.Equal(t, "6", string(body.UnestimatedBacklogStories), "unestimatedBacklogStories")
	var found *projectWarning
	for i := range body.Warnings {
		if body.Warnings[i].Code == "unestimated_backlog_stories" {
			found = &body.Warnings[i]
		}
	}
	if assert.NotNil(t, found, "falta la advertencia unestimated_backlog_stories") {
		assert.Equal(t, "planning", found.Kind)
		if assert.NotNil(t, found.Count, "count de la advertencia") {
			assert.Equal(t, 6, *found.Count)
		}
	}
}

// Escenario: Caso límite — el backlog no tiene historias sin estimar (008/US3)
func TestUS3_BacklogEstimado(t *testing.T) {
	f := newMetricsFixture(t)
	f.accumulatedProject(t)

	body := f.cumulative(t)

	// El contador es 0 y no hay advertencia de backlog (SC-017).
	assert.Equal(t, "0", string(body.UnestimatedBacklogStories), "unestimatedBacklogStories")
	assert.NotNil(t, body.Warnings, "warnings debe existir")
	assert.False(t, hasWarning(body.Warnings, "unestimated_backlog_stories"))
}

// Escenario: Caso de error — identificador de proyecto que no existe (008/US3)
func TestUS3_ProyectoInexistente(t *testing.T) {
	f := newMetricsFixture(t)

	rec := f.env.getProjectMetrics(t, f.ownerToken, uuid.NewString())

	// Mismo 404 y mismo cuerpo que la 002.
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, projectNotFoundBody, rec.Body.String())
}

// activeSprintBody es la respuesta de GET /metrics/active-sprint (contracts/openapi.yaml): metrics es
// un puntero para distinguir el null de "no hay sprint activo" de un objeto vacío.
type activeSprintBody struct {
	ActiveSprint bool               `json:"activeSprint"`
	Metrics      *sprintMetricsBody `json:"metrics"`
}

// getActiveSprint envía GET /api/v1/projects/{projectId}/metrics/active-sprint con el token.
func (e *metricsTestEnv) getActiveSprint(t *testing.T, token, projectID string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, "/api/v1/projects/"+projectID+"/metrics/active-sprint", token, nil)
}

// active consulta las métricas del sprint Activo con la sesión de Ana, exige un 200 con
// activeSprint true y verifica el formato de todas las cifras.
func (f metricsFixture) active(t *testing.T) sprintMetricsBody {
	t.Helper()
	var body activeSprintBody
	metricsRequireStatus(t, f.env.getActiveSprint(t, f.ownerToken, f.projectID.String()), http.StatusOK, &body)
	require.True(t, body.ActiveSprint, "activeSprint debe ser true con un sprint Activo")
	require.NotNil(t, body.Metrics, "metrics no debe ser null con un sprint Activo")
	requireWellFormed(t, *body.Metrics)
	return *body.Metrics
}

// requireAllPartial exige que las nueve métricas lleven la marca partial (FR-014), incluso las que
// no son calculables.
func requireAllPartial(t *testing.T, body sprintMetricsBody) {
	t.Helper()
	requireFinality(t, "plannedStoryPoints", body.PlannedStoryPoints, "partial")
	requireFinality(t, "completedStoryPoints", body.CompletedStoryPoints, "partial")
	requireFinality(t, "estimatedHours", body.EstimatedHours, "partial")
	requireFinality(t, "actualHours", body.ActualHours, "partial")
	requireFinality(t, "deviationHours", body.DeviationHours, "partial")
	requireFinality(t, "deviationPercent", body.DeviationPercent, "partial")
	requireFinality(t, "storyCompletionPercent", body.StoryCompletionPercent, "partial")
	requireFinality(t, "defectsDetected", body.DefectsDetected, "partial")
	requireFinality(t, "defectsResolved", body.DefectsResolved, "partial")
}

// activeSprint arma el "Dado" de US4-1: sprint Activo de 18 Story Points planificados (8, 5 y 5), 5
// completados y 40 horas reales, con factor 6. Devuelve el sprint y sus historias.
func (f metricsFixture) activeSprint(t *testing.T) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	points := []int{5, 8, 5}
	stories := make([]uuid.UUID, 0, len(points))
	for i, p := range points {
		stories = append(stories, f.env.createStory(t, f.ownerToken, f.projectID, "Historia activa "+string(rune('A'+i)), p))
	}
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-12", "2026-10-23", stories...)
	f.env.startSprint(t, f.ownerToken, sprintID)
	f.env.completeStory(t, f.ownerToken, stories[0])
	f.env.logHours(t, f.ownerToken, stories[0], 40)
	return sprintID, stories
}

// Escenario: Caso normal — sprint Activo de 18 puntos planificados, 5 completados y 40 horas (008/US4)
func TestUS4_SprintActivoCasoNormal(t *testing.T) {
	f := newMetricsFixture(t)
	f.activeSprint(t)

	body := f.active(t)

	assert.Equal(t, "active", body.Sprint.Status)
	requireValues(t, body, expectedValues{
		planned: "18", completed: "5", estimated: "108.00", actual: "40.00",
		deviationHours: "-68.00", deviationPercent: "-62.96", completion: "33.33",
	})
	requireValue(t, "defectsDetected", body.DefectsDetected, "0")
	requireValue(t, "defectsResolved", body.DefectsResolved, "0")
	requireAllPartial(t, body)
}

// Escenario: Caso alternativo — al sprint Activo se le suma una historia de 8 Story Points (008/US4)
func TestUS4_HistoriaAgregada(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID, _ := f.activeSprint(t)
	extra := f.env.createStory(t, f.ownerToken, f.projectID, "Historia agregada", 8)
	metricsRequireStatus(t, f.env.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/stories", f.ownerToken,
		map[string]any{"storyIds": []string{extra.String()}}), http.StatusOK, nil)

	body := f.active(t)

	requireValue(t, "plannedStoryPoints", body.PlannedStoryPoints, "26")
	requireValue(t, "estimatedHours", body.EstimatedHours, "156.00")
	requireAllPartial(t, body)
}

// Escenario: Caso alternativo — el propietario cambia el factor de 6 a 8 con un sprint Activo (008/US4)
func TestUS4_FactorVigente(t *testing.T) {
	f := newMetricsFixture(t)
	f.activeSprint(t)
	f.env.changeFactor(t, f.ownerToken, f.projectID, metricsProjectName, 8)

	body := f.active(t)

	requireValue(t, "plannedStoryPoints", body.PlannedStoryPoints, "18")
	requireValue(t, "estimatedHours", body.EstimatedHours, "144.00")
	requireAllPartial(t, body)
}

// Escenario: Caso alternativo — sprint Planificado de 15 Story Points con factor 6 (008/US4)
func TestUS4_SprintPlanificado(t *testing.T) {
	f := newMetricsFixture(t)
	stories := []uuid.UUID{
		f.env.createStory(t, f.ownerToken, f.projectID, "Historia futura A", 8),
		f.env.createStory(t, f.ownerToken, f.projectID, "Historia futura B", 5),
		f.env.createStory(t, f.ownerToken, f.projectID, "Historia futura C", 2),
	}
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint futuro", "2026-12-07", "2026-12-18", stories...)

	body := f.get(t, sprintID)

	// Las dos cifras disponibles no llevan estado de valor y las otras siete no aplican.
	assert.Equal(t, "planned", body.Sprint.Status)
	requireValue(t, "plannedStoryPoints", body.PlannedStoryPoints, "15")
	requireValue(t, "estimatedHours", body.EstimatedHours, "90.00")
	assert.Nil(t, body.PlannedStoryPoints.Finality, "plannedStoryPoints debe llevar finality null")
	assert.Nil(t, body.EstimatedHours.Finality, "estimatedHours debe llevar finality null")
	notApplicable := map[string]metricValue{
		"completedStoryPoints":   body.CompletedStoryPoints,
		"actualHours":            body.ActualHours,
		"deviationHours":         body.DeviationHours,
		"deviationPercent":       body.DeviationPercent,
		"storyCompletionPercent": body.StoryCompletionPercent,
		"defectsDetected":        body.DefectsDetected,
		"defectsResolved":        body.DefectsResolved,
	}
	for name, m := range notApplicable {
		assert.Equal(t, "not_applicable", m.Status, "status de %s", name)
		assert.Equal(t, "null", string(m.Value), "value de %s", name)
		assert.Nil(t, m.Finality, "finality de %s", name)
	}
}

// Escenario: Caso límite — sprint Activo sin ninguna historia (008/US4)
func TestUS4_ActivoSinHistorias(t *testing.T) {
	f := newMetricsFixture(t)
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", "2026-10-12", "2026-10-23")
	f.env.startSprint(t, f.ownerToken, sprintID)

	body := f.active(t)

	requireValues(t, body, expectedValues{
		planned: "0", completed: "0", estimated: "0.00", actual: "0.00",
		deviationHours: "0.00", deviationPercent: notCalculable, completion: notCalculable,
	})
	requireAllPartial(t, body)
}

// Escenario: Caso límite — sprint Activo sin ninguna hora registrada todavía (008/US4)
func TestUS4_ActivoSinHoras(t *testing.T) {
	f := newMetricsFixture(t)
	f.startedSprintN(t, 0, []int{5}, []int{8, 5}, 0)

	body := f.active(t)

	// La desviación absoluta es las horas estimadas en negativo y la porcentual, -100,00 %.
	requireValues(t, body, expectedValues{
		planned: "18", completed: "5", estimated: "108.00", actual: "0.00",
		deviationHours: "-108.00", deviationPercent: "-100.00", completion: "33.33",
	})
	requireAllPartial(t, body)
}

// Escenario: Caso de error — proyecto sin sprint Activo (008/US4)
func TestUS4_SinSprintActivo(t *testing.T) {
	f := newMetricsFixture(t)
	// Un sprint Planificado no cuenta como activo.
	f.plannedSprintWith(t, 0)

	rec := f.env.getActiveSprint(t, f.ownerToken, f.projectID.String())

	metricsRequireStatus(t, rec, http.StatusOK, nil)
	assert.JSONEq(t, `{"activeSprint": false, "metrics": null}`, rec.Body.String())
}
