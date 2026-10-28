package delivery

import (
	"encoding/json"
	"net/http"
	"regexp"
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
