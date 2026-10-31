package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	metricsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/delivery"
	metricsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	metricsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/repository"
	metricsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
	reportsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
)

// Mensajes exactos que fija contracts/openapi.yaml.
const (
	msgSprintNotStarted = "El sprint todavía no arrancó: no tiene nada que reportar."
	msgSprintNotFound   = "El sprint no existe."
)

// Nombres y fechas de los datos de prueba.
const (
	reportsProjectName = "Gestión de Compras — Ñandú"
	reportsSprintStart = "2026-10-12"
	reportsSprintEnd   = "2026-10-23"
)

// reportBody es la parte del cuerpo del reporte que verifican las pruebas; los campos que pueden ser
// null se declaran como punteros o json.RawMessage para distinguir null de ausente.
type reportBody struct {
	Scope   string `json:"scope"`
	Partial bool   `json:"partial"`
	Header  struct {
		ProjectName string `json:"projectName"`
		Sprint      *struct {
			ID       string  `json:"id"`
			Name     string  `json:"name"`
			Status   string  `json:"status"`
			ClosedAt *string `json:"closedAt"`
		} `json:"sprint"`
		Period *struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		} `json:"period"`
		GeneratedAt time.Time `json:"generatedAt"`
		GeneratedBy struct {
			ID       string `json:"id"`
			FullName string `json:"fullName"`
		} `json:"generatedBy"`
	} `json:"header"`
	Stories struct {
		State  string             `json:"state"`
		Groups []reportStoryGroup `json:"groups"`
	} `json:"stories"`
	Estimations struct {
		State  string `json:"state"`
		Groups []struct {
			Sprint *reportGroupSprint `json:"sprint"`
			State  string             `json:"state"`
			Items  []struct {
				Title       string          `json:"title"`
				StoryPoints *int            `json:"storyPoints"`
				Poker       string          `json:"poker"`
				Session     json.RawMessage `json:"session"`
			} `json:"items"`
		} `json:"groups"`
	} `json:"estimations"`
	Effort struct {
		State  string `json:"state"`
		Groups []struct {
			Sprint  *reportGroupSprint `json:"sprint"`
			State   string             `json:"state"`
			Stories []struct {
				Title string  `json:"title"`
				Hours float64 `json:"hours"`
			} `json:"stories"`
			TotalHours float64 `json:"totalHours"`
		} `json:"groups"`
		ByMember []struct {
			FullName string  `json:"fullName"`
			Hours    float64 `json:"hours"`
		} `json:"byMember"`
		TotalHours *float64 `json:"totalHours"`
	} `json:"effort"`
	Metrics struct {
		State   string          `json:"state"`
		Sprint  json.RawMessage `json:"sprint"`
		Project json.RawMessage `json:"project"`
	} `json:"metrics"`
	Defects struct {
		State   string `json:"state"`
		Defects []struct {
			Description string `json:"description"`
			Severity    string `json:"severity"`
			Status      string `json:"status"`
			Story       struct {
				Title string `json:"title"`
			} `json:"story"`
			DetectedSprint struct {
				Name string `json:"name"`
			} `json:"detectedSprint"`
			ResolvedSprint *struct {
				Name string `json:"name"`
			} `json:"resolvedSprint"`
		} `json:"defects"`
		Totals *struct {
			Detected int `json:"detected"`
			Resolved int `json:"resolved"`
		} `json:"totals"`
	} `json:"defects"`
}

// reportGroupSprint es el sprint de un grupo del reporte; es null en el grupo de historias no planificadas.
type reportGroupSprint struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// reportStoryLine es una historia dentro de un grupo de la sección de historias.
type reportStoryLine struct {
	StoryID     string `json:"storyId"`
	Title       string `json:"title"`
	Priority    string `json:"priority"`
	StoryPoints *int   `json:"storyPoints"`
	Status      string `json:"status"`
}

// reportStoryGroup es un grupo de la sección de historias: el de un sprint o el de no planificadas.
type reportStoryGroup struct {
	Sprint  *reportGroupSprint `json:"sprint"`
	State   string             `json:"state"`
	Stories []reportStoryLine  `json:"stories"`
}

// metricValue es una métrica de la 008: estado y valor (null si no está disponible).
type metricValue struct {
	Status string   `json:"status"`
	Value  *float64 `json:"value"`
}

// sprintMetricsBody es la parte del cuerpo de métricas de sprint que verifican las pruebas.
type sprintMetricsBody struct {
	ActualHours            metricValue `json:"actualHours"`
	CompletedStoryPoints   metricValue `json:"completedStoryPoints"`
	DeviationPercent       metricValue `json:"deviationPercent"`
	StoryCompletionPercent metricValue `json:"storyCompletionPercent"`
	DefectsDetected        metricValue `json:"defectsDetected"`
	DefectsResolved        metricValue `json:"defectsResolved"`
}

// reportsFixture reúne el entorno, la propietaria Ana, el integrante Bruno y el proyecto de prueba.
type reportsFixture struct {
	env         *reportsTestEnv
	ownerID     uuid.UUID
	ownerToken  string
	memberID    uuid.UUID
	memberToken string
	projectID   uuid.UUID
}

// newReportsFixture crea el entorno, las dos cuentas y un proyecto En curso con factor 6 al que Bruno
// ya pertenece.
func newReportsFixture(t *testing.T) reportsFixture {
	t.Helper()
	return fixtureOn(t, newReportsTestEnv(t))
}

// fixtureOn crea las cuentas y el proyecto sobre un entorno ya compuesto.
func fixtureOn(t *testing.T, env *reportsTestEnv) reportsFixture {
	t.Helper()
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	memberID, memberToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := env.createProject(t, ownerToken, reportsProjectName, 6)
	env.addMember(t, ownerToken, projectID, "bruno@example.com")
	return reportsFixture{env: env, ownerID: ownerID, ownerToken: ownerToken, memberID: memberID, memberToken: memberToken, projectID: projectID}
}

// activeSprint crea el sprint de la prueba con las historias dadas y lo inicia.
func (f reportsFixture) activeSprint(t *testing.T, stories ...uuid.UUID) uuid.UUID {
	t.Helper()
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", reportsSprintStart, reportsSprintEnd, stories...)
	f.env.startSprint(t, f.ownerToken, sprintID)
	return sprintID
}

// closeAndFixClock cierra el sprint el 23/10 a las 18:00 y deja el reloj en el instante de generación
// devuelto, el 24/10 a las 10:30:15.
func (f reportsFixture) closeAndFixClock(t *testing.T, sprintID uuid.UUID) time.Time {
	t.Helper()
	f.env.closeSprintAt(t, f.ownerToken, sprintID, f.env.at(2026, time.October, 23, 18, 0))
	generatedAt := time.Date(2026, time.October, 24, 10, 30, 15, 0, f.env.location)
	f.env.clock.Set(generatedAt)
	return generatedAt
}

// report genera el reporte del sprint como la propietaria, exige el 200 y lo decodifica.
func (f reportsFixture) report(t *testing.T, sprintID uuid.UUID) reportBody {
	t.Helper()
	return f.reportAs(t, f.ownerToken, sprintID)
}

// reportAs genera el reporte del sprint con el token indicado, exige el 200 y lo decodifica.
func (f reportsFixture) reportAs(t *testing.T, token string, sprintID uuid.UUID) reportBody {
	t.Helper()
	var body reportBody
	requireStatus(t, f.env.generateSprintReport(t, token, f.projectID, sprintID), http.StatusOK, &body)
	return body
}

// metricsReference devuelve el cuerpo de GET /api/v1/projects/{id}/metrics/sprints/{id} de la 008.
func (f reportsFixture) metricsReference(t *testing.T, sprintID uuid.UUID) json.RawMessage {
	t.Helper()
	return f.metricsGet(t, "/metrics/sprints/"+sprintID.String())
}

// projectMetricsReference devuelve el cuerpo de GET /api/v1/projects/{id}/metrics de la 008.
func (f reportsFixture) projectMetricsReference(t *testing.T) json.RawMessage {
	t.Helper()
	return f.metricsGet(t, "/metrics")
}

// metricsGet pide a la 008 la ruta indicada, relativa al proyecto, y exige el 200. El entorno de
// reportes no registra esas rutas, por eso se compone un router mínimo con los mismos servicios, la
// misma base y el mismo reloj.
func (f reportsFixture) metricsGet(t *testing.T, suffix string) json.RawMessage {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err, "no se pudo cargar la configuración de test")
	db := f.env.db
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	router := gin.New()
	authHandler := authdelivery.NewHandler(auth)
	projectRepository := projectsrepository.NewProjectRepository(db)
	projectMembers := projectsrepository.NewProjectMemberRepository(db)
	projects := projectsservice.NewProjectService(projectRepository, projectMembers,
		projectsrepository.NewBacklogSummaryProvider(db), projectsrepository.NewSprintDirectory(db))
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(db), f.env.location, f.env.clock.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, authHandler.RequireSession,
		projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)
	metrics := metricsservice.NewMetricsService(metricsrepository.NewMetricsRepository(db), reportsGrace, f.env.clock.Now)
	metricsdelivery.RegisterRoutes(projectGroup, metricsdelivery.NewHandler(metrics))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+f.projectID.String()+suffix, nil)
	req.Header.Set("Authorization", "Bearer "+f.ownerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "métricas de referencia: %s", rec.Body.String())
	return json.RawMessage(rec.Body.Bytes())
}

// errorMessage exige el código indicado y devuelve el mensaje de error del cuerpo.
func errorMessage(t *testing.T, rec *httptest.ResponseRecorder, code int) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	requireStatus(t, rec, code, &body)
	return body.Error
}

// Escenario: Caso normal — sprint Cerrado con 4 historias, esfuerzo y 3 defectos (010/US1)
func TestUS1_ReporteDeSprintCerrado(t *testing.T) {
	f := newReportsFixture(t)
	env := f.env
	alta := env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	baja := env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
	listado := env.createStory(t, f.ownerToken, f.projectID, "Listado de clientes", "low", 3)
	exportacion := env.createStory(t, f.ownerToken, f.projectID, "Exportación de clientes", "high", 2)
	sprintID := f.activeSprint(t, alta, baja, listado, exportacion)
	env.completeStory(t, f.ownerToken, alta)
	env.completeStory(t, f.ownerToken, baja)

	// Esfuerzo de dos integrantes: 14,25 horas en Alta (10 y 4,25) y 6,5 en Baja.
	env.logHours(t, f.ownerToken, alta, 10)
	env.logHours(t, f.memberToken, alta, 4.25)
	env.logHours(t, f.ownerToken, baja, 6.5)

	// Cuatro defectos: uno resuelto en el sprint, uno abierto, uno en curso y uno Descartado.
	resolved := env.createDefect(t, f.ownerToken, f.projectID, alta, sprintID, "critical", "Falla el alta con CUIT repetido")
	env.createDefect(t, f.ownerToken, f.projectID, baja, sprintID, "high", "La baja no libera el CUIT")
	inProgress := env.createDefect(t, f.ownerToken, f.projectID, listado, sprintID, "low", "Texto cortado en el listado")
	discarded := env.createDefect(t, f.ownerToken, f.projectID, exportacion, sprintID, "medium", "Defecto duplicado")
	env.changeDefectStatus(t, f.ownerToken, resolved, "in_progress", uuid.Nil)
	env.changeDefectStatus(t, f.ownerToken, resolved, "resolved", sprintID)
	env.changeDefectStatus(t, f.ownerToken, inProgress, "in_progress", uuid.Nil)
	env.changeDefectStatus(t, f.ownerToken, discarded, "discarded", uuid.Nil)
	generatedAt := f.closeAndFixClock(t, sprintID)

	body := f.reportAs(t, f.memberToken, sprintID)

	// Alcance, marca de parcial y encabezado.
	assert.Equal(t, "sprint", body.Scope)
	assert.False(t, body.Partial, "un sprint cerrado no es parcial")
	assert.Equal(t, reportsProjectName, body.Header.ProjectName)
	require.NotNil(t, body.Header.Sprint)
	assert.Equal(t, "Sprint 1", body.Header.Sprint.Name)
	assert.Equal(t, "closed", body.Header.Sprint.Status)
	require.NotNil(t, body.Header.Period)
	assert.Equal(t, reportsSprintStart, body.Header.Period.StartDate)
	assert.Equal(t, reportsSprintEnd, body.Header.Period.EndDate)
	assert.True(t, generatedAt.Equal(body.Header.GeneratedAt), "generatedAt = %s, se esperaba %s", body.Header.GeneratedAt, generatedAt)
	assert.Equal(t, "Bruno Gómez", body.Header.GeneratedBy.FullName)
	assert.Equal(t, f.memberID.String(), body.Header.GeneratedBy.ID)

	// Historias en orden de prioridad y título, con el resultado de la instantánea.
	assert.Equal(t, "complete", body.Stories.State)
	require.Len(t, body.Stories.Groups, 1)
	lines := body.Stories.Groups[0].Stories
	require.Len(t, lines, 4)
	type wantLine struct {
		title, priority, status string
		points                  int
	}
	wantLines := []wantLine{
		{"Alta de clientes", "high", "completed", 8},
		{"Exportación de clientes", "high", "not_completed", 2},
		{"Baja de clientes", "medium", "completed", 5},
		{"Listado de clientes", "low", "not_completed", 3},
	}
	for i, want := range wantLines {
		assert.Equal(t, want.title, lines[i].Title, "título de la historia %d", i)
		assert.Equal(t, want.priority, lines[i].Priority, "prioridad de %q", want.title)
		assert.Equal(t, want.status, lines[i].Status, "estado de %q", want.title)
		require.NotNil(t, lines[i].StoryPoints, "Story Points de %q", want.title)
		assert.Equal(t, want.points, *lines[i].StoryPoints, "Story Points de %q", want.title)
	}

	// Estimaciones: las cuatro historias, con sus puntos.
	assert.Equal(t, "complete", body.Estimations.State)
	require.Len(t, body.Estimations.Groups, 1)
	assert.Len(t, body.Estimations.Groups[0].Items, 4)

	// Esfuerzo por historia y por integrante, con el total.
	assert.Equal(t, "complete", body.Effort.State)
	require.Len(t, body.Effort.Groups, 1)
	byStory := map[string]float64{}
	for _, line := range body.Effort.Groups[0].Stories {
		byStory[line.Title] = line.Hours
	}
	assert.Equal(t, map[string]float64{"Alta de clientes": 14.25, "Baja de clientes": 6.5}, byStory)
	assert.InDelta(t, 20.75, body.Effort.Groups[0].TotalHours, 0.001)
	byMember := map[string]float64{}
	for _, line := range body.Effort.ByMember {
		byMember[line.FullName] = line.Hours
	}
	assert.Equal(t, map[string]float64{"Ana Pérez": 16.5, "Bruno Gómez": 4.25}, byMember)
	require.NotNil(t, body.Effort.TotalHours)
	assert.InDelta(t, 20.75, *body.Effort.TotalHours, 0.001)

	// Métricas: el cuerpo de la 008 sin recalcular.
	assert.Equal(t, "complete", body.Metrics.State)
	require.JSONEq(t, string(f.metricsReference(t, sprintID)), string(body.Metrics.Sprint))

	// Defectos: los tres vigentes, sin el Descartado, con sus columnas y los totales de la 008.
	assert.Equal(t, "complete", body.Defects.State)
	require.Len(t, body.Defects.Defects, 3)
	byDescription := map[string]int{}
	for i, defect := range body.Defects.Defects {
		byDescription[defect.Description] = i
		assert.Equal(t, "Sprint 1", defect.DetectedSprint.Name)
	}
	assert.NotContains(t, byDescription, "Defecto duplicado")
	critical := body.Defects.Defects[byDescription["Falla el alta con CUIT repetido"]]
	assert.Equal(t, "critical", critical.Severity)
	assert.Equal(t, "resolved", critical.Status)
	assert.Equal(t, "Alta de clientes", critical.Story.Title)
	require.NotNil(t, critical.ResolvedSprint)
	assert.Equal(t, "Sprint 1", critical.ResolvedSprint.Name)
	open := body.Defects.Defects[byDescription["La baja no libera el CUIT"]]
	assert.Equal(t, "open", open.Status)
	assert.Nil(t, open.ResolvedSprint)
	require.NotNil(t, body.Defects.Totals)
	var reference sprintMetricsBody
	require.NoError(t, json.Unmarshal(f.metricsReference(t, sprintID), &reference))
	require.NotNil(t, reference.DefectsDetected.Value)
	require.NotNil(t, reference.DefectsResolved.Value)
	assert.Equal(t, int(*reference.DefectsDetected.Value), body.Defects.Totals.Detected)
	assert.Equal(t, int(*reference.DefectsResolved.Value), body.Defects.Totals.Resolved)
	assert.Equal(t, 3, body.Defects.Totals.Detected)
	assert.Equal(t, 1, body.Defects.Totals.Resolved)
}

// Escenario: Caso alternativo — sprint Activo (010/US1)
func TestUS1_SprintActivoParcial(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)

	body := f.report(t, sprintID)

	// El reporte se genera igual y queda marcado como parcial.
	assert.Equal(t, "sprint", body.Scope)
	assert.True(t, body.Partial, "el reporte del sprint activo debe ser parcial")
	require.NotNil(t, body.Header.Sprint)
	assert.Equal(t, "active", body.Header.Sprint.Status)
	assert.Nil(t, body.Header.Sprint.ClosedAt)
}

// Escenario: Caso alternativo — historia con sesión de Planning Poker Finalizada de 3 rondas y valor 8 (010/US1)
func TestUS1_HistoriaConPlanningPoker(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	agreed := 8
	closedAt := f.env.at(2026, time.October, 20, 10, 0)
	f.env.seedPokerSession(t, story, pokerdomain.SessionStatusFinished, 3, &agreed, f.env.at(2026, time.October, 20, 9, 0), &closedAt)

	body := f.report(t, sprintID)

	require.Len(t, body.Estimations.Groups, 1)
	require.Len(t, body.Estimations.Groups[0].Items, 1)
	item := body.Estimations.Groups[0].Items[0]
	assert.Equal(t, "agreed", item.Poker)
	require.NotNil(t, item.StoryPoints)
	assert.Equal(t, 8, *item.StoryPoints)
	var session struct {
		Rounds            int `json:"rounds"`
		AgreedStoryPoints int `json:"agreedStoryPoints"`
	}
	require.NoError(t, json.Unmarshal(item.Session, &session), "session = %s", item.Session)
	assert.Equal(t, 3, session.Rounds)
	assert.Equal(t, 8, session.AgreedStoryPoints)
}

// Escenario: Caso alternativo — historia que nunca pasó por Planning Poker (010/US1)
func TestUS1_HistoriaSinPlanningPoker(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 5)
	sprintID := f.activeSprint(t, story)

	body := f.report(t, sprintID)

	// La fila no queda vacía: lleva los puntos y indica que no hubo sesión.
	require.Len(t, body.Estimations.Groups, 1)
	require.Len(t, body.Estimations.Groups[0].Items, 1)
	item := body.Estimations.Groups[0].Items[0]
	assert.Equal(t, "none", item.Poker)
	require.NotNil(t, item.StoryPoints)
	assert.Equal(t, 5, *item.StoryPoints)
	assert.JSONEq(t, "null", string(item.Session), "session debe ser null")
}

// Escenario: Caso límite — sprint Cerrado sin ninguna historia completada (010/US1)
func TestUS1_SinHistoriasCompletadas(t *testing.T) {
	f := newReportsFixture(t)
	first := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	second := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
	sprintID := f.activeSprint(t, first, second)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// Las historias planificadas se listan, ninguna Completada, y las métricas lo reflejan.
	require.Len(t, body.Stories.Groups, 1)
	require.Len(t, body.Stories.Groups[0].Stories, 2)
	for _, line := range body.Stories.Groups[0].Stories {
		assert.Equal(t, "not_completed", line.Status, "estado de %q", line.Title)
	}
	var metrics sprintMetricsBody
	require.NoError(t, json.Unmarshal(body.Metrics.Sprint, &metrics))
	assert.Equal(t, "available", metrics.CompletedStoryPoints.Status)
	require.NotNil(t, metrics.CompletedStoryPoints.Value)
	assert.Equal(t, 0.0, *metrics.CompletedStoryPoints.Value)
}

// Escenario: Caso límite — sprint Cerrado sin ninguna historia (010/US1)
func TestUS1_SprintSinHistorias(t *testing.T) {
	f := newReportsFixture(t)
	sprintID := f.activeSprint(t)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// Las secciones que dependen de historias van "sin datos"; encabezado y métricas siguen presentes.
	assert.Equal(t, "empty", body.Stories.State)
	assert.Equal(t, "empty", body.Estimations.State)
	assert.Equal(t, reportsProjectName, body.Header.ProjectName)
	assert.Equal(t, "complete", body.Metrics.State)
	assert.NotEmpty(t, body.Metrics.Sprint, "las métricas deben estar presentes")
}

// Escenario: Caso límite — valor no calculable, nunca 0 (010/US1)
func TestUS1_MetricaNoCalculable(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 0)
	sprintID := f.activeSprint(t, story)
	f.env.logHours(t, f.ownerToken, story, 3)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// Con 0 horas estimadas, la desviación porcentual no es calculable y no vale 0.
	var metrics sprintMetricsBody
	require.NoError(t, json.Unmarshal(body.Metrics.Sprint, &metrics))
	assert.Equal(t, "not_calculable", metrics.DeviationPercent.Status)
	assert.Nil(t, metrics.DeviationPercent.Value, "value debe ser null")
}

// Escenario: Caso de error — sprint Planificado (010/US1)
func TestUS1_SprintPlanificado(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 1", reportsSprintStart, reportsSprintEnd, story)

	rec := f.env.generateSprintReport(t, f.ownerToken, f.projectID, sprintID)

	assert.Equal(t, msgSprintNotStarted, errorMessage(t, rec, http.StatusConflict))
	assert.Empty(t, f.env.generationRows(t, f.projectID), "no debe dejar registro")
}

// Escenario: Caso de error — sprint de otro proyecto (010/US1)
func TestUS1_SprintDeOtroProyecto(t *testing.T) {
	f := newReportsFixture(t)
	otherProject := f.env.createProject(t, f.ownerToken, "Otro proyecto", 6)
	foreignStory := f.env.createStory(t, f.ownerToken, otherProject, "Historia ajena", "high", 8)
	foreignSprint := f.env.createSprint(t, f.ownerToken, otherProject, "Sprint ajeno", reportsSprintStart, reportsSprintEnd, foreignStory)
	f.env.startSprint(t, f.ownerToken, foreignSprint)

	foreign := f.env.generateSprintReport(t, f.ownerToken, f.projectID, foreignSprint)
	unknown := f.env.generateSprintReport(t, f.ownerToken, f.projectID, uuid.New())

	// La respuesta es idéntica a la de un sprint inexistente y no deja registro.
	assert.Equal(t, msgSprintNotFound, errorMessage(t, foreign, http.StatusNotFound))
	assert.Equal(t, unknown.Body.String(), foreign.Body.String())
	assert.Empty(t, f.env.generationRows(t, f.projectID), "no debe dejar registro")
	assert.Empty(t, f.env.generationRows(t, otherProject), "no debe dejar registro")
}

// Escenario: Caso de error — usuario que no es integrante (010/US1)
func TestUS1_NoIntegrante(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	_, outsiderToken := f.env.signUp(t, "Carla Ruiz", "carla@example.com")

	outsider := f.env.generateSprintReport(t, outsiderToken, f.projectID, sprintID)
	missing := f.env.generateSprintReport(t, outsiderToken, uuid.New(), sprintID)

	// La respuesta es idéntica a la de un proyecto inexistente y no deja registro.
	require.Equal(t, http.StatusNotFound, outsider.Code, "cuerpo: %s", outsider.Body.String())
	assert.Equal(t, missing.Body.String(), outsider.Body.String())
	assert.Empty(t, f.env.generationRows(t, f.projectID), "no debe dejar registro")
}

func TestReportRequiresSession(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)

	// Sin token, las dos rutas responden 401 y no dejan registro.
	assert.Equal(t, http.StatusUnauthorized, f.env.generateSprintReport(t, "", f.projectID, sprintID).Code)
	assert.Equal(t, http.StatusUnauthorized, f.env.generateProjectReport(t, "", f.projectID).Code)
	assert.Empty(t, f.env.generationRows(t, f.projectID))
}

// countingReportRepository cuenta las lecturas que recibe el repositorio de reportes.
type countingReportRepository struct {
	reportsdomain.ReportRepository
	calls *atomic.Int32
}

func (c countingReportRepository) LoadHeader(ctx context.Context, projectID, userID uuid.UUID) (reportsdomain.ProjectInfo, reportsdomain.UserInfo, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadHeader(ctx, projectID, userID)
}

func (c countingReportRepository) LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (reportsdomain.SprintRef, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadSprint(ctx, projectID, sprintID)
}

func (c countingReportRepository) ListSprints(ctx context.Context, projectID uuid.UUID) ([]reportsdomain.SprintRef, error) {
	c.calls.Add(1)
	return c.ReportRepository.ListSprints(ctx, projectID)
}

func (c countingReportRepository) LoadSprintStories(ctx context.Context, sprint reportsdomain.SprintRef) ([]reportsdomain.StoryLine, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadSprintStories(ctx, sprint)
}

func (c countingReportRepository) LoadUnplannedStories(ctx context.Context, projectID uuid.UUID) ([]reportsdomain.StoryLine, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadUnplannedStories(ctx, projectID)
}

func (c countingReportRepository) LoadEstimationSessions(ctx context.Context, storyIDs []uuid.UUID) (map[uuid.UUID][]reportsdomain.SessionRecord, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadEstimationSessions(ctx, storyIDs)
}

func (c countingReportRepository) LoadEffort(ctx context.Context, sprintIDs []uuid.UUID) ([]reportsdomain.EffortRow, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadEffort(ctx, sprintIDs)
}

func (c countingReportRepository) LoadDefects(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID) ([]reportsdomain.DefectLine, error) {
	c.calls.Add(1)
	return c.ReportRepository.LoadDefects(ctx, projectID, sprintID)
}

func TestReportMalformedSprintID(t *testing.T) {
	var calls atomic.Int32
	env := newReportsTestEnvWith(t, func(repo reportsdomain.ReportRepository, metrics reportsdomain.MetricsSource) (reportsdomain.ReportRepository, reportsdomain.MetricsSource) {
		return countingReportRepository{ReportRepository: repo, calls: &calls}, metrics
	})
	f := fixtureOn(t, env)

	rec := env.do(t, http.MethodPost, "/api/v1/projects/"+f.projectID.String()+"/reports/sprints/no-es-un-uuid", f.ownerToken, nil)

	// Responde como sprint inexistente sin consultar el repositorio ni dejar registro.
	assert.Equal(t, msgSprintNotFound, errorMessage(t, rec, http.StatusNotFound))
	assert.Zero(t, calls.Load(), "no debe consultar el repositorio con un identificador mal formado")
	assert.Empty(t, env.generationRows(t, f.projectID))
}

func TestReportEffortMatchesMetrics(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	f.env.logHours(t, f.ownerToken, story, 7.5)
	f.env.logHours(t, f.memberToken, story, 2.25)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// El total de esfuerzo es el de las horas reales de la sección de métricas.
	var metrics sprintMetricsBody
	require.NoError(t, json.Unmarshal(body.Metrics.Sprint, &metrics))
	require.NotNil(t, metrics.ActualHours.Value)
	require.NotNil(t, body.Effort.TotalHours)
	assert.InDelta(t, *metrics.ActualHours.Value, *body.Effort.TotalHours, 0.001)
	assert.InDelta(t, 9.75, *body.Effort.TotalHours, 0.001)
}

// tableStamp es la fila de una tabla con su fecha de última actualización.
type tableStamp struct {
	ID        string
	UpdatedAt time.Time
}

// stamps lee el identificador y updated_at de todas las filas de la tabla.
func (e *reportsTestEnv) stamps(t *testing.T, table string) []tableStamp {
	t.Helper()
	var rows []tableStamp
	require.NoError(t, e.db.Raw("SELECT id::text AS id, updated_at FROM "+table+" ORDER BY id").Scan(&rows).Error,
		"no se pudo leer %s", table)
	return rows
}

func TestReportWritesOnlyGeneration(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	f.env.logHours(t, f.ownerToken, story, 4)
	f.env.createDefect(t, f.ownerToken, f.projectID, story, sprintID, "high", "Falla el alta")
	f.closeAndFixClock(t, sprintID)
	tables := []string{"projects", "user_stories", "sprints", "defects", "effort_entries"}
	before := map[string][]tableStamp{}
	for _, table := range tables {
		before[table] = f.env.stamps(t, table)
	}
	require.Empty(t, f.env.generationRows(t, f.projectID))

	f.report(t, sprintID)

	// Ninguna fila de las demás tablas cambió ni se agregó (RN-02); solo se sumó la generación.
	for _, table := range tables {
		assert.Equal(t, before[table], f.env.stamps(t, table), "la tabla %s no debe cambiar", table)
	}
	assert.Len(t, f.env.generationRows(t, f.projectID), 1)
}

func TestReportGenerationRow(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	generatedAt := f.closeAndFixClock(t, sprintID)

	body := f.reportAs(t, f.memberToken, sprintID)

	// Una fila por 200, atribuida a quien pidió el reporte y con el instante del encabezado.
	rows := f.env.generationRows(t, f.projectID)
	require.Len(t, rows, 1)
	assert.Equal(t, reportsdomain.ScopeSprint, rows[0].Scope)
	require.NotNil(t, rows[0].SprintID)
	assert.Equal(t, sprintID, *rows[0].SprintID)
	assert.Equal(t, f.memberID, rows[0].GeneratedByID)
	assert.True(t, body.Header.GeneratedAt.Equal(rows[0].GeneratedAt), "generated_at = %s, header = %s", rows[0].GeneratedAt, body.Header.GeneratedAt)
	assert.True(t, generatedAt.Equal(rows[0].GeneratedAt), "generated_at = %s, reloj = %s", rows[0].GeneratedAt, generatedAt)
}

// Mensaje del 404 de la 002 para un proyecto inexistente.
const msgProjectNotFound = "El proyecto no existe."

// unplannedLabel nombra, en las listas de nombres de grupo, al grupo de historias no planificadas, que
// no tiene sprint.
const unplannedLabel = "(no planificadas)"

// projectReport genera el reporte del proyecto como la propietaria, exige el 200 y lo decodifica.
func (f reportsFixture) projectReport(t *testing.T) reportBody {
	t.Helper()
	var body reportBody
	requireStatus(t, f.env.generateProjectReport(t, f.ownerToken, f.projectID), http.StatusOK, &body)
	return body
}

// closedSprint crea el sprint con las historias dadas, carga las horas indicadas (si son más de 0) a
// nombre de la propietaria sobre la primera historia, lo inicia, completa las historias indicadas y lo
// cierra un día después del instante del reloj. El reloj solo avanza, para que cada sprint empiece
// después de que cerró el anterior.
func (f reportsFixture) closedSprint(t *testing.T, name, start, end string, hours float64, completed []uuid.UUID, stories ...uuid.UUID) uuid.UUID {
	t.Helper()
	sprintID := f.env.createSprint(t, f.ownerToken, f.projectID, name, start, end, stories...)
	if hours > 0 {
		f.env.logHours(t, f.ownerToken, stories[0], hours)
	}
	f.env.startSprint(t, f.ownerToken, sprintID)
	for _, storyID := range completed {
		f.env.completeStory(t, f.ownerToken, storyID)
	}
	f.env.closeSprintAt(t, f.ownerToken, sprintID, f.env.clock.Now().Add(24*time.Hour))
	return sprintID
}

// threeClosedSprints deja tres sprints Cerrados, con una historia cada uno y nombres que no siguen el
// orden de las fechas: "Sprint Gamma" (1 al 7 de octubre), "Sprint Alfa" (8 al 14) y "Sprint Beta"
// (15 al 21). Devuelve las tres historias en ese orden.
func (f reportsFixture) threeClosedSprints(t *testing.T) [3]uuid.UUID {
	t.Helper()
	alta := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	baja := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
	listado := f.env.createStory(t, f.ownerToken, f.projectID, "Listado de clientes", "low", 3)
	f.closedSprint(t, "Sprint Gamma", "2026-10-01", "2026-10-07", 4, []uuid.UUID{alta}, alta)
	f.closedSprint(t, "Sprint Alfa", "2026-10-08", "2026-10-14", 3, []uuid.UUID{baja}, baja)
	f.closedSprint(t, "Sprint Beta", "2026-10-15", "2026-10-21", 2, nil, listado)
	return [3]uuid.UUID{alta, baja, listado}
}

// storyGroupNames devuelve el nombre del sprint de cada grupo de historias, en orden; el grupo sin
// sprint figura como unplannedLabel.
func storyGroupNames(groups []reportStoryGroup) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.Sprint == nil {
			names = append(names, unplannedLabel)
			continue
		}
		names = append(names, group.Sprint.Name)
	}
	return names
}

// storyTitles devuelve los títulos de las historias de un grupo, en orden.
func storyTitles(lines []reportStoryLine) []string {
	titles := make([]string, 0, len(lines))
	for _, line := range lines {
		titles = append(titles, line.Title)
	}
	return titles
}

// projectMetricsBody es la parte del cuerpo de métricas del proyecto que verifican las pruebas.
type projectMetricsBody struct {
	Cumulative struct {
		ActualHours metricValue `json:"actualHours"`
	} `json:"cumulative"`
}

// Escenario: Caso normal — proyecto con 3 sprints cerrados (010/US3)
func TestUS3_ProyectoConTresSprintsCerrados(t *testing.T) {
	f := newReportsFixture(t)
	f.threeClosedSprints(t)
	generatedAt := time.Date(2026, time.October, 30, 10, 0, 0, 0, f.env.location)
	f.env.clock.Set(generatedAt)

	body := f.projectReport(t)

	// Alcance, encabezado y período del primer inicio al último fin; sin marca de parcial.
	assert.Equal(t, "project", body.Scope)
	assert.False(t, body.Partial, "sin sprint Activo el reporte no es parcial")
	assert.Equal(t, reportsProjectName, body.Header.ProjectName)
	assert.Nil(t, body.Header.Sprint, "header.sprint es null en el alcance project")
	require.NotNil(t, body.Header.Period)
	assert.Equal(t, "2026-10-01", body.Header.Period.StartDate)
	assert.Equal(t, "2026-10-21", body.Header.Period.EndDate)
	assert.True(t, generatedAt.Equal(body.Header.GeneratedAt), "generatedAt = %s", body.Header.GeneratedAt)

	// Historias: un grupo por sprint en orden cronológico y, al final, el de no planificadas vacío.
	assert.Equal(t, "complete", body.Stories.State)
	assert.Equal(t, []string{"Sprint Gamma", "Sprint Alfa", "Sprint Beta", unplannedLabel}, storyGroupNames(body.Stories.Groups))
	require.Len(t, body.Stories.Groups, 4)
	assert.Equal(t, []string{"Alta de clientes"}, storyTitles(body.Stories.Groups[0].Stories))
	assert.Equal(t, []string{"Baja de clientes"}, storyTitles(body.Stories.Groups[1].Stories))
	assert.Equal(t, []string{"Listado de clientes"}, storyTitles(body.Stories.Groups[2].Stories))
	assert.Equal(t, "completed", body.Stories.Groups[0].Stories[0].Status)
	assert.Equal(t, "not_completed", body.Stories.Groups[2].Stories[0].Status)
	assert.Equal(t, "empty", body.Stories.Groups[3].State)

	// Estimaciones: los mismos grupos, en el mismo orden.
	assert.Equal(t, "complete", body.Estimations.State)
	require.Len(t, body.Estimations.Groups, 4)
	for i, name := range []string{"Sprint Gamma", "Sprint Alfa", "Sprint Beta"} {
		require.NotNil(t, body.Estimations.Groups[i].Sprint)
		assert.Equal(t, name, body.Estimations.Groups[i].Sprint.Name)
		assert.Len(t, body.Estimations.Groups[i].Items, 1)
	}
	assert.Nil(t, body.Estimations.Groups[3].Sprint)

	// Esfuerzo: un grupo por sprint con sus horas y el total del proyecto.
	assert.Equal(t, "complete", body.Effort.State)
	require.Len(t, body.Effort.Groups, 3)
	for i, hours := range []float64{4, 3, 2} {
		assert.InDelta(t, hours, body.Effort.Groups[i].TotalHours, 0.001, "horas del grupo %d", i)
	}
	require.NotNil(t, body.Effort.TotalHours)
	assert.InDelta(t, 9, *body.Effort.TotalHours, 0.001)

	// Métricas: el cuerpo de GET /projects/{id}/metrics de la 008 sin recalcular.
	assert.Equal(t, "complete", body.Metrics.State)
	assert.JSONEq(t, "null", string(body.Metrics.Sprint), "metrics.sprint es null en el alcance project")
	require.JSONEq(t, string(f.projectMetricsReference(t)), string(body.Metrics.Project))

	// Una fila de generación con el alcance project y sin sprint.
	rows := f.env.generationRows(t, f.projectID)
	require.Len(t, rows, 1)
	assert.Equal(t, reportsdomain.ScopeProject, rows[0].Scope)
	assert.Nil(t, rows[0].SprintID)
}

// Escenario: Caso alternativo — proyecto con 3 sprints cerrados y uno Activo (010/US3)
func TestUS3_ProyectoConSprintActivo(t *testing.T) {
	f := newReportsFixture(t)
	f.threeClosedSprints(t)
	delta := f.env.createStory(t, f.ownerToken, f.projectID, "Exportación de clientes", "high", 2)
	activeID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint Delta", "2026-10-22", "2026-10-28", delta)
	f.env.startSprint(t, f.ownerToken, activeID)

	body := f.projectReport(t)

	// Los cuatro sprints en orden, el Activo último, y la marca de parcial.
	assert.True(t, body.Partial, "el reporte con un sprint Activo debe ser parcial")
	assert.Equal(t, []string{"Sprint Gamma", "Sprint Alfa", "Sprint Beta", "Sprint Delta", unplannedLabel}, storyGroupNames(body.Stories.Groups))
	require.Len(t, body.Stories.Groups, 5)
	require.NotNil(t, body.Stories.Groups[3].Sprint)
	assert.Equal(t, "active", body.Stories.Groups[3].Sprint.Status)
	assert.Len(t, body.Effort.Groups, 4, "un grupo de esfuerzo por cada uno de los 4 sprints")
	require.NotNil(t, body.Header.Period)
	assert.Equal(t, "2026-10-01", body.Header.Period.StartDate)
	assert.Equal(t, "2026-10-28", body.Header.Period.EndDate)
}

// Escenario: Caso alternativo — proyecto con un sprint Planificado además de los cerrados (010/US3)
func TestUS3_SprintPlanificadoExcluido(t *testing.T) {
	f := newReportsFixture(t)
	env := f.env
	visible := env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	onlyPlanned := env.createStory(t, f.ownerToken, f.projectID, "Historia solo en el planificado", "medium", 5)
	closedID := f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 5, []uuid.UUID{visible}, visible)
	env.createDefect(t, f.ownerToken, f.projectID, visible, closedID, "high", "Falla visible del cerrado")
	env.createSprint(t, f.ownerToken, f.projectID, "Sprint 2", "2026-10-08", "2026-10-14", onlyPlanned)

	// Esfuerzo anclado al Planificado; la 007 no permite detectar defectos en un sprint Planificado.
	env.logHours(t, f.ownerToken, onlyPlanned, 3)

	body := f.projectReport(t)

	// Ningún grupo del Planificado en historias, estimaciones ni esfuerzo.
	assert.Equal(t, []string{"Sprint 1", unplannedLabel}, storyGroupNames(body.Stories.Groups))
	require.Len(t, body.Estimations.Groups, 2)
	assert.Len(t, body.Effort.Groups, 1)
	require.NotNil(t, body.Header.Period)
	assert.Equal(t, "2026-10-07", body.Header.Period.EndDate)

	// Su historia aparece entre las no planificadas con el estado vigente.
	require.Len(t, body.Stories.Groups, 2)
	unplanned := body.Stories.Groups[1]
	require.Len(t, unplanned.Stories, 1)
	assert.Equal(t, "Historia solo en el planificado", unplanned.Stories[0].Title)
	assert.Equal(t, "pending", unplanned.Stories[0].Status)

	// Sin las horas del Planificado; el listado trae solo el defecto real.
	require.NotNil(t, body.Effort.TotalHours)
	assert.InDelta(t, 5, *body.Effort.TotalHours, 0.001)
	require.Len(t, body.Defects.Defects, 1)
	assert.Equal(t, "Falla visible del cerrado", body.Defects.Defects[0].Description)
}

// Escenario: Caso alternativo — proyecto Finalizado (010/US3)
func TestUS3_ProyectoFinalizado(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 0, []uuid.UUID{story}, story)
	f.env.finishProject(t, f.ownerToken, f.projectID)

	rec := f.env.generateProjectReport(t, f.memberToken, f.projectID)

	// Se genera completo, sin marca de parcial, y deja su registro (RN-20).
	var body reportBody
	requireStatus(t, rec, http.StatusOK, &body)
	assert.False(t, body.Partial, "el proyecto Finalizado no es parcial")
	assert.Equal(t, "project", body.Scope)
	rows := f.env.generationRows(t, f.projectID)
	require.Len(t, rows, 1)
	assert.Equal(t, reportsdomain.ScopeProject, rows[0].Scope)
	assert.Equal(t, f.memberID, rows[0].GeneratedByID)
}

// Escenario: Caso límite — historia planificada en dos sprints (010/US3)
func TestUS3_HistoriaEnDosSprints(t *testing.T) {
	f := newReportsFixture(t)
	replanned := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 5)
	done := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 3)
	f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 0, []uuid.UUID{done}, replanned, done)
	f.closedSprint(t, "Sprint 2", "2026-10-08", "2026-10-14", 0, []uuid.UUID{replanned}, replanned)

	body := f.projectReport(t)

	// La historia está en los dos grupos, con el resultado que tuvo en cada uno (RN-08).
	assert.Equal(t, []string{"Sprint 1", "Sprint 2", unplannedLabel}, storyGroupNames(body.Stories.Groups))
	require.Len(t, body.Stories.Groups, 3)
	first := body.Stories.Groups[0].Stories
	require.Len(t, first, 2)
	assert.Equal(t, "Alta de clientes", first[0].Title)
	assert.Equal(t, "not_completed", first[0].Status)
	assert.Equal(t, "Baja de clientes", first[1].Title)
	assert.Equal(t, "completed", first[1].Status)
	second := body.Stories.Groups[1].Stories
	require.Len(t, second, 1)
	assert.Equal(t, "Alta de clientes", second[0].Title)
	assert.Equal(t, first[0].StoryID, second[0].StoryID)
	assert.Equal(t, "completed", second[0].Status)
}

// Escenario: Caso alternativo — 12 historias que no entraron a ningún sprint (010/US3)
func TestUS3_HistoriasNoPlanificadas(t *testing.T) {
	f := newReportsFixture(t)
	env := f.env
	planned := env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 0, []uuid.UUID{planned}, planned)
	wantTitles := []string{"Historia 12"}
	for i := 1; i <= 12; i++ {
		title := fmt.Sprintf("Historia %02d", i)
		priority, points := "medium", 5
		switch i {
		case 12:
			priority = "high"
		case 11:
			points = -1
		}
		env.createStory(t, f.ownerToken, f.projectID, title, priority, points)
		if i != 12 {
			wantTitles = append(wantTitles, title)
		}
	}

	body := f.projectReport(t)

	// El grupo final no tiene sprint, trae las 12 con sus datos vigentes y en orden de prioridad y título.
	require.Len(t, body.Stories.Groups, 2)
	last := body.Stories.Groups[1]
	assert.Nil(t, last.Sprint)
	assert.Equal(t, "complete", last.State)
	require.Equal(t, wantTitles, storyTitles(last.Stories))
	assert.Equal(t, "high", last.Stories[0].Priority)
	for _, line := range last.Stories {
		assert.Equal(t, "pending", line.Status, "estado de %q", line.Title)
		if line.Title == "Historia 11" {
			assert.Nil(t, line.StoryPoints, "la historia sin estimar lleva storyPoints null")
		} else {
			require.NotNil(t, line.StoryPoints, "Story Points de %q", line.Title)
			assert.Equal(t, 5, *line.StoryPoints)
		}
	}
	require.Len(t, body.Estimations.Groups, 2)
	assert.Len(t, body.Estimations.Groups[1].Items, 12)
}

// Escenario: Caso límite — todas las historias estuvieron en algún sprint (010/US3)
func TestUS3_SinHistoriasNoPlanificadas(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 0, []uuid.UUID{story}, story)

	body := f.projectReport(t)

	// El grupo de no planificadas se muestra igual, sin historias.
	require.Len(t, body.Stories.Groups, 2)
	last := body.Stories.Groups[1]
	assert.Nil(t, last.Sprint)
	assert.Equal(t, "empty", last.State)
	assert.Empty(t, last.Stories)
	require.Len(t, body.Estimations.Groups, 2)
	assert.Equal(t, "empty", body.Estimations.Groups[1].State)
}

// closedWithTwoStories deja un sprint Cerrado con dos historias sin completar, "Alta de clientes"
// (alta, 8 puntos) y "Baja de clientes" (media, 5), con 6 horas cargadas, y devuelve la primera. El
// reloj queda un día después del cierre.
func (f reportsFixture) closedWithTwoStories(t *testing.T) uuid.UUID {
	t.Helper()
	alta := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	baja := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
	f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 6, nil, alta, baja)
	return alta
}

// changeAfterClose renombra, cambia la prioridad y reestima la historia, y registra una sesión de
// Planning Poker Finalizada con 13 puntos acordados después del cierre del sprint.
func (f reportsFixture) changeAfterClose(t *testing.T, storyID uuid.UUID) {
	t.Helper()
	// La sesión se abrió antes del cierre del sprint (cuenta dentro del corte) y se Finalizó después.
	createdAt := f.env.clock.Now().Add(-time.Minute)
	f.env.clock.Advance(2 * time.Hour)
	f.env.renameStory(t, f.ownerToken, storyID, "Alta renombrada")
	f.env.changePriority(t, f.ownerToken, storyID, "low")
	f.env.estimateStory(t, f.ownerToken, storyID, 13)
	agreed := 13
	closedAt := f.env.clock.Now()
	f.env.seedPokerSession(t, storyID, pokerdomain.SessionStatusFinished, 2, &agreed, createdAt, &closedAt)
}

// Escenario: Caso límite — historias renombradas y reestimadas después del cierre (010/US3)
func TestUS3_HistoriasCambiadasDespuesDelCierre(t *testing.T) {
	f := newReportsFixture(t)
	alta := f.closedWithTwoStories(t)
	f.changeAfterClose(t, alta)

	body := f.projectReport(t)

	// El grupo del Cerrado muestra el título, la prioridad y los Story Points del cierre.
	require.Len(t, body.Stories.Groups, 2)
	lines := body.Stories.Groups[0].Stories
	require.Len(t, lines, 2)
	assert.Equal(t, "Alta de clientes", lines[0].Title)
	assert.Equal(t, "high", lines[0].Priority)
	require.NotNil(t, lines[0].StoryPoints)
	assert.Equal(t, 8, *lines[0].StoryPoints)
	assert.Equal(t, "not_completed", lines[0].Status)
	assert.Equal(t, "Baja de clientes", lines[1].Title)

	// La estimación es la de antes del corte: la sesión Finalizada después del cierre no cuenta.
	require.Len(t, body.Estimations.Groups, 2)
	items := body.Estimations.Groups[0].Items
	require.Len(t, items, 2)
	assert.Equal(t, "Alta de clientes", items[0].Title)
	require.NotNil(t, items[0].StoryPoints)
	assert.Equal(t, 8, *items[0].StoryPoints)
	assert.Equal(t, "no_agreement", items[0].Poker)
	assert.JSONEq(t, "null", string(items[0].Session), "session debe ser null")
	assert.Equal(t, "none", items[1].Poker)
}

// Escenario: Caso límite — dos reportes del mismo sprint cerrado con un mes de diferencia (010/US3)
func TestUS3_ReporteReproducible(t *testing.T) {
	f := newReportsFixture(t)
	alta := f.closedWithTwoStories(t)

	// La primera generación es posterior al plazo de gracia de 48 horas, para que la finalidad de las
	// métricas no cambie por el solo paso del tiempo.
	f.env.clock.Advance(3 * 24 * time.Hour)
	first := decodeReport(t, f.env.generateProjectReport(t, f.ownerToken, f.projectID))

	// Cambios en el backlog y, un mes después, la segunda generación la hace otra persona.
	f.changeAfterClose(t, alta)
	f.env.clock.Advance(30 * 24 * time.Hour)
	second := decodeReport(t, f.env.generateProjectReport(t, f.memberToken, f.projectID))

	// Solo difieren la fecha, la hora y quien lo generó (RN-22, SC-017).
	assert.NotEqual(t, first["header"].(map[string]any)["generatedAt"], second["header"].(map[string]any)["generatedAt"])
	assert.Equal(t, withoutGeneration(first), withoutGeneration(second))
	assert.Len(t, f.env.generationRows(t, f.projectID), 2)
}

// Escenario: Caso límite — proyecto sin ningún sprint cerrado ni activo (010/US3)
func TestUS3_ProyectoSinSprints(t *testing.T) {
	t.Run("sin sprints ni historias", func(t *testing.T) {
		f := newReportsFixture(t)

		body := f.projectReport(t)

		// Encabezado completo y las cinco secciones sin datos; no es un error.
		assert.Equal(t, "project", body.Scope)
		assert.False(t, body.Partial)
		assert.Equal(t, reportsProjectName, body.Header.ProjectName)
		assert.Nil(t, body.Header.Sprint)
		assert.Nil(t, body.Header.Period, "period debe ser null sin sprints reportables")
		assert.Equal(t, "Ana Pérez", body.Header.GeneratedBy.FullName)
		assert.False(t, body.Header.GeneratedAt.IsZero())
		assert.Equal(t, "empty", body.Stories.State)
		assert.Equal(t, "empty", body.Estimations.State)
		assert.Equal(t, "empty", body.Effort.State)
		assert.Equal(t, "empty", body.Metrics.State)
		assert.Equal(t, "empty", body.Defects.State)
		assert.Len(t, f.env.generationRows(t, f.projectID), 1)
	})

	t.Run("sin sprints y con 3 historias", func(t *testing.T) {
		f := newReportsFixture(t)
		f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
		f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
		f.env.createStory(t, f.ownerToken, f.projectID, "Listado de clientes", "low", -1)

		body := f.projectReport(t)

		// Las 3 historias van en el grupo de no planificadas, sin sesiones de estimación.
		assert.Nil(t, body.Header.Period)
		assert.Equal(t, "complete", body.Stories.State)
		require.Len(t, body.Stories.Groups, 1)
		assert.Nil(t, body.Stories.Groups[0].Sprint)
		assert.Len(t, body.Stories.Groups[0].Stories, 3)
		assert.Equal(t, "complete", body.Estimations.State)
		require.Len(t, body.Estimations.Groups, 1)
		require.Len(t, body.Estimations.Groups[0].Items, 3)
		for _, item := range body.Estimations.Groups[0].Items {
			assert.Equal(t, "none", item.Poker, "poker de %q", item.Title)
		}

		// Esfuerzo, métricas y defectos siguen sin datos.
		assert.Equal(t, "empty", body.Effort.State)
		require.NotNil(t, body.Effort.TotalHours)
		assert.Zero(t, *body.Effort.TotalHours)
		assert.Equal(t, "empty", body.Metrics.State)
		assert.Equal(t, "empty", body.Defects.State)
	})
}

// Escenario: Caso de error — proyecto inexistente (010/US3)
func TestUS3_ProyectoInexistente(t *testing.T) {
	f := newReportsFixture(t)
	missingID := uuid.New()

	rec := f.env.generateProjectReport(t, f.ownerToken, missingID)

	// Es el 404 de la 002, idéntico al del reporte de sprint, y no deja registro.
	assert.Equal(t, msgProjectNotFound, errorMessage(t, rec, http.StatusNotFound))
	assert.Equal(t, f.env.generateSprintReport(t, f.ownerToken, missingID, uuid.New()).Body.String(), rec.Body.String())
	assert.Empty(t, f.env.generationRows(t, missingID))
}

func TestProjectReportEffortMatchesMetrics(t *testing.T) {
	f := newReportsFixture(t)
	first := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	second := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
	f.closedSprint(t, "Sprint 1", "2026-10-01", "2026-10-07", 7.5, nil, first)
	activeID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint 2", "2026-10-08", "2026-10-14", second)
	f.env.startSprint(t, f.ownerToken, activeID)
	f.env.logHours(t, f.memberToken, second, 2.25)

	body := f.projectReport(t)

	// El total de esfuerzo es el de las horas reales acumuladas de la sección de métricas.
	var metrics projectMetricsBody
	require.NoError(t, json.Unmarshal(body.Metrics.Project, &metrics), "metrics.project = %s", body.Metrics.Project)
	require.NotNil(t, metrics.Cumulative.ActualHours.Value)
	require.NotNil(t, body.Effort.TotalHours)
	assert.InDelta(t, *metrics.Cumulative.ActualHours.Value, *body.Effort.TotalHours, 0.001)
	assert.InDelta(t, 9.75, *body.Effort.TotalHours, 0.001)
}

func TestProjectReportActiveSprintChanges(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	f.activeSprint(t, story)
	before := f.projectReport(t)

	f.env.renameStory(t, f.ownerToken, story, "Alta renombrada")
	after := f.projectReport(t)

	// Con un sprint Activo los datos son los vigentes: el cambio se ve en el reporte siguiente (RN-23).
	require.Len(t, before.Stories.Groups, 2)
	require.Len(t, after.Stories.Groups, 2)
	assert.Equal(t, []string{"Alta de clientes"}, storyTitles(before.Stories.Groups[0].Stories))
	assert.Equal(t, []string{"Alta renombrada"}, storyTitles(after.Stories.Groups[0].Stories))
}

// Escenario: Caso límite — sprint sin ningún registro de esfuerzo (010/US4)
func TestUS4_SprintSinEsfuerzo(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// La sección figura sin datos, con el total en 0.00 y los grupos sin filas.
	assert.Equal(t, "empty", body.Effort.State)
	require.NotNil(t, body.Effort.TotalHours)
	assert.Zero(t, *body.Effort.TotalHours)
	assert.Empty(t, body.Effort.ByMember)
	for _, group := range body.Effort.Groups {
		assert.Empty(t, group.Stories, "el grupo no debe tener filas")
		assert.Zero(t, group.TotalHours)
	}
}

// Escenario: Caso límite — sprint sin ningún defecto (010/US4)
func TestUS4_SprintSinDefectos(t *testing.T) {
	f := newReportsFixture(t)
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// La sección figura sin datos pero conserva los totales, en 0.
	assert.Equal(t, "empty", body.Defects.State)
	assert.Empty(t, body.Defects.Defects)
	require.NotNil(t, body.Defects.Totals)
	assert.Zero(t, body.Defects.Totals.Detected)
	assert.Zero(t, body.Defects.Totals.Resolved)
}

// Escenario: Caso límite — historias sin estimar (010/US4)
func TestUS4_HistoriasSinEstimar(t *testing.T) {
	f := newReportsFixture(t)
	first := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	second := f.env.createStory(t, f.ownerToken, f.projectID, "Baja de clientes", "medium", 5)
	sprintID := f.activeSprint(t, first, second)
	// La 004 no deja planificar historias sin estimar, así que se les quitan los puntos ya dentro del sprint.
	f.env.forceRow(t, "UPDATE user_stories SET story_points = NULL WHERE id IN (?, ?)", first, second)
	f.closeAndFixClock(t, sprintID)

	body := f.report(t, sprintID)

	// Las historias se listan sin Story Points y las métricas son las de la 008.
	require.Len(t, body.Stories.Groups, 1)
	require.Len(t, body.Stories.Groups[0].Stories, 2)
	for _, line := range body.Stories.Groups[0].Stories {
		assert.Nil(t, line.StoryPoints, "Story Points de %q", line.Title)
	}
	assert.Equal(t, "complete", body.Metrics.State)
	require.JSONEq(t, string(f.metricsReference(t, sprintID)), string(body.Metrics.Sprint))
}

// Escenario: Caso límite — proyecto sin sprints ni historias (010/US4)
func TestUS4_SeccionesSinDatos(t *testing.T) {
	f := newReportsFixture(t)

	body := f.projectReport(t)

	// Las cinco secciones figuran sin datos y el encabezado está completo.
	assert.Equal(t, reportsProjectName, body.Header.ProjectName)
	assert.Equal(t, "Ana Pérez", body.Header.GeneratedBy.FullName)
	assert.False(t, body.Header.GeneratedAt.IsZero())
	assert.Equal(t, "empty", body.Stories.State)
	assert.Equal(t, "empty", body.Estimations.State)
	assert.Equal(t, "empty", body.Effort.State)
	assert.Equal(t, "empty", body.Metrics.State)
	assert.Equal(t, "empty", body.Defects.State)
}

// simulatedFailure es el texto del error inyectado; no debe aparecer nunca en una respuesta.
const simulatedFailure = "fallo simulado de la fuente"

// failingMetricsSource hace fallar las dos lecturas de la fuente de métricas.
type failingMetricsSource struct {
	reportsdomain.MetricsSource
}

func (failingMetricsSource) SprintMetrics(context.Context, uuid.UUID, uuid.UUID) (metricsdomain.SprintMetrics, []metricsdomain.Warning, error) {
	return metricsdomain.SprintMetrics{}, nil, errors.New(simulatedFailure)
}

func (failingMetricsSource) ProjectMetrics(context.Context, uuid.UUID) (metricsdomain.ProjectMetrics, error) {
	return metricsdomain.ProjectMetrics{}, errors.New(simulatedFailure)
}

// failingEffortRepository hace fallar solo la lectura del esfuerzo.
type failingEffortRepository struct {
	reportsdomain.ReportRepository
}

func (failingEffortRepository) LoadEffort(context.Context, []uuid.UUID) ([]reportsdomain.EffortRow, error) {
	return nil, errors.New(simulatedFailure)
}

// failingHeaderRepository hace fallar solo la lectura del encabezado.
type failingHeaderRepository struct {
	reportsdomain.ReportRepository
}

func (failingHeaderRepository) LoadHeader(context.Context, uuid.UUID, uuid.UUID) (reportsdomain.ProjectInfo, reportsdomain.UserInfo, error) {
	return reportsdomain.ProjectInfo{}, reportsdomain.UserInfo{}, errors.New(simulatedFailure)
}

// isJSONNull indica si el valor JSON crudo falta o es null.
func isJSONNull(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// failingFixture arma un sprint Cerrado con historia, horas y defecto sobre un entorno compuesto.
func failingFixture(t *testing.T, compose reportsCompose) (reportsFixture, uuid.UUID) {
	t.Helper()
	f := fixtureOn(t, newReportsTestEnvWith(t, compose))
	story := f.env.createStory(t, f.ownerToken, f.projectID, "Alta de clientes", "high", 8)
	sprintID := f.activeSprint(t, story)
	f.env.logHours(t, f.ownerToken, story, 4)
	f.env.createDefect(t, f.ownerToken, f.projectID, story, sprintID, "high", "Falla el alta")
	f.closeAndFixClock(t, sprintID)
	return f, sprintID
}

// Escenario: Caso de error — una sección no se pudo obtener (010/US4)
func TestUS4_SeccionNoObtenible(t *testing.T) {
	t.Run("falla la fuente de métricas", func(t *testing.T) {
		f, sprintID := failingFixture(t, func(repo reportsdomain.ReportRepository, metrics reportsdomain.MetricsSource) (reportsdomain.ReportRepository, reportsdomain.MetricsSource) {
			return repo, failingMetricsSource{MetricsSource: metrics}
		})

		body := f.report(t, sprintID)

		// Métricas y defectos no se obtuvieron, con sus datos en null; el resto sigue completo.
		assert.Equal(t, "unavailable", body.Metrics.State)
		assert.True(t, isJSONNull(body.Metrics.Sprint), "metrics.sprint = %s", body.Metrics.Sprint)
		assert.True(t, isJSONNull(body.Metrics.Project), "metrics.project = %s", body.Metrics.Project)
		assert.Equal(t, "unavailable", body.Defects.State)
		assert.Nil(t, body.Defects.Defects)
		assert.Nil(t, body.Defects.Totals)
		assert.Equal(t, "complete", body.Stories.State)
		assert.Equal(t, "complete", body.Estimations.State)
		assert.Equal(t, "complete", body.Effort.State)
		assert.Len(t, f.env.generationRows(t, f.projectID), 1)
	})

	t.Run("falla solo la lectura del esfuerzo", func(t *testing.T) {
		f, sprintID := failingFixture(t, func(repo reportsdomain.ReportRepository, metrics reportsdomain.MetricsSource) (reportsdomain.ReportRepository, reportsdomain.MetricsSource) {
			return failingEffortRepository{ReportRepository: repo}, metrics
		})

		body := f.report(t, sprintID)

		// Solo el esfuerzo queda sin obtener, con sus datos en null; las demás secciones no se ven afectadas.
		assert.Equal(t, "unavailable", body.Effort.State)
		assert.Nil(t, body.Effort.Groups)
		assert.Nil(t, body.Effort.ByMember)
		assert.Nil(t, body.Effort.TotalHours)
		assert.Equal(t, "complete", body.Stories.State)
		assert.Equal(t, "complete", body.Estimations.State)
		assert.Equal(t, "complete", body.Metrics.State)
		assert.Equal(t, "complete", body.Defects.State)
		assert.Len(t, f.env.generationRows(t, f.projectID), 1)
	})

	t.Run("falla la lectura del encabezado", func(t *testing.T) {
		f, sprintID := failingFixture(t, func(repo reportsdomain.ReportRepository, metrics reportsdomain.MetricsSource) (reportsdomain.ReportRepository, reportsdomain.MetricsSource) {
			return failingHeaderRepository{ReportRepository: repo}, metrics
		})

		rec := f.env.generateSprintReport(t, f.ownerToken, f.projectID, sprintID)

		// Sin encabezado no hay reporte: 500 genérico, sin detalles y sin registro de generación.
		require.Equal(t, http.StatusInternalServerError, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.NotContains(t, rec.Body.String(), simulatedFailure)
		assert.Empty(t, f.env.generationRows(t, f.projectID), "no debe dejar registro")
	})
}
