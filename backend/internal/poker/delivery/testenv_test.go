package delivery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	backlogdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/delivery"
	backlogrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	backlogservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/service"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
	sprintsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/delivery"
	sprintsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
	sprintsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Entorno de las pruebas de integración del Planning Poker (T015). Compone el router como cmd/app
// —sesión de la 001, proyectos de la 002, backlog de la 003, sprints de la 004 y Planning Poker—
// detrás de httptest.NewServer, con un reloj falso que la prueba avanza y acceso al servicio para
// llamar a Sweep a mano, así ninguna prueba espera 30 segundos, 5 minutos ni 60 minutos reales
// (research.md sección 17). El canal de eventos se lee con openStream, sobre net/http y bufio. Los
// ayudantes llevan el prefijo "poker" para no chocar con los de middleware_test.go, que llevan el
// prefijo "estimationAccess".

// pokerStreamDeadline es el plazo en que debe llegar cada instantánea por el canal: el límite de 2
// segundos de FR-073.
const pokerStreamDeadline = 2 * time.Second

// pokerHTTPTimeout acota cada petición REST de las pruebas, para que una respuesta que no llega
// falle la prueba en lugar de colgarla.
const pokerHTTPTimeout = 5 * time.Second

// pokerPassword es la contraseña de las cuentas de prueba; cumple la política de la 001.
const pokerPassword = "Poker2026"

// pokerFixedNow es el instante inicial del reloj falso que reciben el servicio de Planning Poker y
// el de sprints.
var pokerFixedNow = time.Date(2026, time.October, 23, 15, 0, 0, 0, time.UTC)

// pokerClock es el reloj falso inyectado en el servicio: devuelve siempre el mismo instante hasta
// que la prueba lo avanza. Es seguro para uso concurrente porque lo leen los manejadores del
// servidor de prueba.
type pokerClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now devuelve el instante vigente del reloj falso.
func (c *pokerClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance adelanta el reloj falso la duración indicada.
func (c *pokerClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// pokerTestEnv reúne el servidor de prueba, la base de test con su configuración, el reloj falso y
// el servicio de sesiones sobre el que la prueba llama a Sweep. connClosed registra, por dirección
// del cliente, un canal que se cierra cuando el servidor termina de atender esa conexión.
type pokerTestEnv struct {
	server   *httptest.Server
	db       *gorm.DB
	cfg      config.Config
	clock    *pokerClock
	sessions *service.SessionService

	connMu     sync.Mutex
	connClosed map[string]chan struct{}
}

// newPokerTestEnv abre la base de test vacía, compone el router con newPokerRouter y lo levanta con
// httptest. El servidor se cierra al terminar la prueba, después de los canales que la prueba haya
// dejado abiertos.
func newPokerTestEnv(t *testing.T) *pokerTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	clock := &pokerClock{now: pokerFixedNow}
	env := &pokerTestEnv{db: db, cfg: cfg, clock: clock, connClosed: map[string]chan struct{}{}}
	env.start()
	t.Cleanup(env.stop)
	return env
}

// start compone un router nuevo, con su propio servicio de sesiones y su presencia vacía, y lo
// levanta detrás de httptest. El servidor anota cuándo termina de atender cada conexión, para que
// cerrar un canal espere a que el manejador haya liberado la conexión antes de que la prueba avance
// el reloj.
func (e *pokerTestEnv) start() {
	router, sessions := newPokerRouter(e.db, e.cfg, e.clock)
	server := httptest.NewUnstartedServer(router)
	server.Config.ConnState = e.trackConn
	server.Start()
	e.server = server
	e.sessions = sessions
}

// stop corta las conexiones abiertas con el servidor vigente y lo cierra.
func (e *pokerTestEnv) stop() {
	e.server.CloseClientConnections()
	e.server.Close()
}

// restart simula el reinicio del servidor como cmd/app: cierra el servidor vigente, con lo que se
// cortan todos los canales, ejecuta ResetPresence con el instante del reloj falso y levanta un
// servidor nuevo cuyo servicio arranca con la presencia vacía (research.md sección 9).
func (e *pokerTestEnv) restart(t *testing.T) {
	t.Helper()
	e.stop()
	require.NoError(t, repository.NewSessionRepository(e.db).ResetPresence(context.Background(), e.clock.Now()),
		"ResetPresence falló")
	e.start()
}

// newPokerRouter compone, igual que cmd/app, el límite del cuerpo, el rechazo de queries mal
// formadas, la cadena de autenticación de la 001, el módulo de proyectos de la 002 con su subgrupo
// /:projectId, el backlog de la 003 con su grupo de historia, los sprints de la 004 y el Planning
// Poker, sin CORS ni Swagger, y devuelve el router con el servicio de sesiones sobre el que la prueba
// llama a Sweep.
func newPokerRouter(db *gorm.DB, cfg config.Config, clock *pokerClock) (*gin.Engine, *service.SessionService) {
	// Cadena de autenticación de la 001.
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	router := gin.New()
	router.Use(httpserver.LimitRequestBody())
	router.Use(httpserver.RejectMalformedQuery())
	authHandler := authdelivery.NewHandler(auth)
	authdelivery.RegisterRoutes(router, authHandler)

	// Módulo de proyectos de la 002, que devuelve el subgrupo /api/v1/projects/:projectId.
	projectRepository := projectsrepository.NewProjectRepository(db)
	projectMembers := projectsrepository.NewProjectMemberRepository(db)
	projects := projectsservice.NewProjectService(
		projectRepository,
		projectMembers,
		projectsrepository.NewBacklogSummaryProvider(db),
		projectsrepository.NewSprintDirectory(db),
	)
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(db), cfg.Location, time.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, authHandler.RequireSession,
		projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// Módulo del backlog de la 003, con el grupo plano de historia protegido por RequireStoryAccess.
	storyRepository := backlogrepository.NewStoryRepository(db)
	stories := backlogservice.NewStoryService(
		storyRepository,
		backlogrepository.NewSprintAssignments(db),
		backlogrepository.NewEffortRecords(),
		backlogrepository.NewDefectRecords(),
		backlogrepository.NewEstimationSessions(),
		backlogrepository.NewProjectDirectory(db),
	)
	backlogHandler := backlogdelivery.NewHandler(stories)
	backlogdelivery.RegisterRoutes(projectGroup, backlogHandler)
	backlogdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, storyRepository, backlogHandler)

	// Módulo de sprints de la 004 con el reloj falso, para llevar una historia a Completada.
	sprintRepository := sprintsrepository.NewSprintRepository(db)
	sprints := sprintsservice.NewSprintService(sprintRepository, sprintsrepository.NewSprintEffort(), cfg.Location, clock.Now)
	sprintsHandler := sprintsdelivery.NewHandler(sprints)
	sprintsdelivery.RegisterRoutes(projectGroup, sprintsHandler)
	sprintsdelivery.RegisterSprintRoutes(router, authHandler.RequireSession, sprintRepository, sprintsHandler)
	sprintsdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), sprintsHandler)

	// Módulo de Planning Poker con el reloj falso: la apertura cuelga de la historia con el
	// RequireStoryAccess de la 003 y el resto del grupo plano de la sesión, sobre la misma instancia
	// del repositorio que usa el servicio.
	sessionRepository := repository.NewSessionRepository(db)
	sessions := service.NewSessionService(sessionRepository, clock.Now)
	pokerHandler := NewHandler(sessions)
	RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), pokerHandler)
	RegisterSessionRoutes(router, authHandler.RequireSession, sessionRepository, pokerHandler)
	return router, sessions
}

// trackConn cierra el canal de la conexión cuando el servidor deja de atenderla.
func (e *pokerTestEnv) trackConn(conn net.Conn, state http.ConnState) {
	if state != http.StateClosed && state != http.StateHijacked {
		return
	}
	ch := e.connChannel(conn.RemoteAddr().String())
	e.connMu.Lock()
	defer e.connMu.Unlock()
	select {
	case <-ch:
	default:
		close(ch)
	}
}

// connChannel devuelve, creándolo si no existe, el canal que se cierra cuando termina la conexión
// con la dirección de cliente indicada.
func (e *pokerTestEnv) connChannel(addr string) chan struct{} {
	e.connMu.Lock()
	defer e.connMu.Unlock()
	ch, ok := e.connClosed[addr]
	if !ok {
		ch = make(chan struct{})
		e.connClosed[addr] = ch
	}
	return ch
}

// pokerResponse es una respuesta HTTP ya leída: código, tipo de contenido y cuerpo.
type pokerResponse struct {
	Code        int
	ContentType string
	Body        []byte
}

// do envía una petición al servidor de prueba con el cuerpo JSON opcional y el token como Bearer
// (si no es vacío), y devuelve la respuesta leída por completo.
func (e *pokerTestEnv) do(t *testing.T, method, path, token string, body any) pokerResponse {
	t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err, "no se pudo serializar la solicitud")
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	require.NoError(t, err, "no se pudo armar la solicitud %s %s", method, path)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: pokerHTTPTimeout}
	resp, err := client.Do(req)
	require.NoError(t, err, "la solicitud %s %s no obtuvo respuesta", method, path)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "no se pudo leer la respuesta de %s %s", method, path)
	return pokerResponse{Code: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: data}
}

// requireStatus exige el código indicado y, si hay destino, decodifica el cuerpo JSON en él.
func requirePokerStatus(t *testing.T, resp pokerResponse, code int, target any) {
	t.Helper()
	require.Equal(t, code, resp.Code, "cuerpo: %s", resp.Body)
	if target != nil {
		require.NoError(t, json.Unmarshal(resp.Body, target), "el cuerpo no es JSON válido: %s", resp.Body)
	}
}

// signUp registra una cuenta con POST /api/v1/auth/register de la 001, inicia su sesión con POST
// /api/v1/auth/login y devuelve el identificador de la cuenta y el token.
func (e *pokerTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	var profile struct {
		ID string `json:"id"`
	}
	requirePokerStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/register", "",
		map[string]any{"fullName": fullName, "email": email, "password": pokerPassword}), http.StatusCreated, &profile)
	var login struct {
		Token string `json:"token"`
	}
	requirePokerStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"email": email, "password": pokerPassword}), http.StatusOK, &login)
	id, err := uuid.Parse(profile.ID)
	require.NoError(t, err, "id = %q no es un UUID", profile.ID)
	return id, login.Token
}

// createProject crea con POST /api/v1/projects de la 002 un proyecto Planificado del titular del
// token y devuelve su identificador.
func (e *pokerTestEnv) createProject(t *testing.T, ownerToken, name string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	requirePokerStatus(t, e.do(t, http.MethodPost, "/api/v1/projects", ownerToken, map[string]any{
		"name":               name,
		"startDate":          "2026-10-01",
		"plannedEndDate":     "2026-12-20",
		"hoursPerStoryPoint": 6.5,
	}), http.StatusCreated, &created)
	id, err := uuid.Parse(created.ID)
	require.NoError(t, err, "id = %q no es un UUID", created.ID)
	return id
}

// setProjectStatus cambia el estado del proyecto con PATCH /api/v1/projects/{id}/status de la 002.
func (e *pokerTestEnv) setProjectStatus(t *testing.T, ownerToken string, projectID uuid.UUID, status string) {
	t.Helper()
	requirePokerStatus(t, e.do(t, http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/status", ownerToken,
		map[string]any{"status": status}), http.StatusOK, nil)
}

// addMember suma al proyecto, con POST /api/v1/projects/{id}/members de la 002, la cuenta del
// correo indicado.
func (e *pokerTestEnv) addMember(t *testing.T, ownerToken string, projectID uuid.UUID, email string) {
	t.Helper()
	requirePokerStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/members", ownerToken,
		map[string]any{"email": email}), http.StatusCreated, nil)
}

// pokerStoryInput son los datos de una historia que se crea con la 003; Points nil es "sin
// estimar" y Description nil es sin descripción.
type pokerStoryInput struct {
	Title       string
	Description *string
	Points      *int
	Criteria    []string
}

// createStory crea la historia en el backlog del proyecto con POST
// /api/v1/projects/{id}/stories de la 003 y devuelve su identificador.
func (e *pokerTestEnv) createStory(t *testing.T, token string, projectID uuid.UUID, in pokerStoryInput) uuid.UUID {
	t.Helper()
	criteria := in.Criteria
	if criteria == nil {
		criteria = []string{}
	}
	body := map[string]any{
		"title":              in.Title,
		"description":        in.Description,
		"priority":           "high",
		"storyPoints":        in.Points,
		"acceptanceCriteria": criteria,
	}
	var created struct {
		ID string `json:"id"`
	}
	requirePokerStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stories", token, body),
		http.StatusCreated, &created)
	id, err := uuid.Parse(created.ID)
	require.NoError(t, err, "id = %q no es un UUID", created.ID)
	return id
}

// openSessionPath es la ruta de apertura de una sesión sobre la historia según
// contracts/openapi.yaml.
func openSessionPath(storyID uuid.UUID) string {
	return "/api/v1/stories/" + storyID.String() + "/estimation-sessions"
}

// eventsPath es la ruta del canal de eventos de la sesión según contracts/openapi.yaml.
func eventsPath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/events"
}

// openSession pide POST /api/v1/stories/{storyId}/estimation-sessions con el token como Bearer.
func (e *pokerTestEnv) openSession(t *testing.T, token string, storyID uuid.UUID) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPost, openSessionPath(storyID), token, nil)
}

// requireOpenedSession abre una sesión sobre la historia, exige el 201 y devuelve la sesión
// decodificada.
func (e *pokerTestEnv) requireOpenedSession(t *testing.T, token string, storyID uuid.UUID) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.openSession(t, token, storyID)
	requirePokerStatus(t, resp, http.StatusCreated, &session)
	session.Raw = resp.Body
	return session
}

// sweep avanza el reloj falso la duración indicada y ejecuta una pasada del barrido.
func (e *pokerTestEnv) sweep(t *testing.T, advance time.Duration) {
	t.Helper()
	e.clock.Advance(advance)
	require.NoError(t, e.sessions.Sweep(context.Background()), "el barrido falló")
}

// pokerUserRef refleja UserRef del contrato.
type pokerUserRef struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
}

// pokerParticipantBody refleja EstimationParticipant del contrato.
type pokerParticipantBody struct {
	User          pokerUserRef `json:"user"`
	Connected     bool         `json:"connected"`
	IsFacilitator bool         `json:"isFacilitator"`
	HasVoted      bool         `json:"hasVoted"`
}

// pokerRoundBody refleja EstimationRound del contrato, con los campos que leen las pruebas de US1.
type pokerRoundBody struct {
	Number     int     `json:"number"`
	Status     string  `json:"status"`
	OpenedAt   string  `json:"openedAt"`
	RevealedAt *string `json:"revealedAt"`
}

// pokerStoryBody refleja EstimationStory del contrato.
type pokerStoryBody struct {
	ID                 string   `json:"id"`
	ProjectID          string   `json:"projectId"`
	Title              string   `json:"title"`
	Description        *string  `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

// pokerSessionBody refleja EstimationSession del contrato: el cuerpo de la apertura y de cada evento
// state del canal. Se declara en la prueba para no depender de los DTO de producción. Raw guarda el
// JSON recibido, para comprobar que un campo vale [] o null y no que simplemente falta.
type pokerSessionBody struct {
	ID                     string                 `json:"id"`
	Version                int64                  `json:"version"`
	Status                 string                 `json:"status"`
	Story                  pokerStoryBody         `json:"story"`
	Facilitator            pokerUserRef           `json:"facilitator"`
	OpenedBy               pokerUserRef           `json:"openedBy"`
	OpenedAt               string                 `json:"openedAt"`
	ClosedBy               *pokerUserRef          `json:"closedBy"`
	ClosedAt               *string                `json:"closedAt"`
	AgreedStoryPoints      *int                   `json:"agreedStoryPoints"`
	CancelReason           *string                `json:"cancelReason"`
	Cards                  []string               `json:"cards"`
	Participants           []pokerParticipantBody `json:"participants"`
	Rounds                 []pokerRoundBody       `json:"rounds"`
	MyVote                 *string                `json:"myVote"`
	FacilitatorAvailableAt *string                `json:"facilitatorAvailableAt"`
	Raw                    []byte                 `json:"-"`
}

// participant devuelve el participante con el identificador indicado, o nil si no figura.
func (s pokerSessionBody) participant(userID uuid.UUID) *pokerParticipantBody {
	for i := range s.Participants {
		if s.Participants[i].User.ID == userID.String() {
			return &s.Participants[i]
		}
	}
	return nil
}

// pokerStreamEvent es un evento leído del canal: su nombre (campo event) y sus datos (las líneas
// data unidas con salto de línea).
type pokerStreamEvent struct {
	Name string
	Data []byte
}

// pokerStream es el lector de un canal de eventos abierto: un goroutine lee el cuerpo línea por
// línea con bufio y entrega cada evento completo por events. Close corta la conexión y espera a que
// el servidor termine de atenderla.
type pokerStream struct {
	env       *pokerTestEnv
	cancel    context.CancelFunc
	body      io.ReadCloser
	events    chan pokerStreamEvent
	stop      chan struct{}
	done      chan struct{}
	localAddr string
	closeOnce sync.Once
}

// openStream abre GET /api/v1/estimation-sessions/{sessionId}/events con el token como Bearer en
// una conexión propia, exige el 200 con Content-Type text/event-stream dentro del plazo de 2
// segundos y devuelve el lector. El canal se cierra solo al terminar la prueba si la prueba no lo
// cerró antes.
func (e *pokerTestEnv) openStream(t *testing.T, token, sessionID string) *pokerStream {
	t.Helper()

	// Cada canal usa su propia conexión, y se anota la dirección local para reconocerla del lado
	// del servidor.
	var localAddr string
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
			if err == nil {
				localAddr = conn.LocalAddr().String()
			}
			return conn, err
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.server.URL+eventsPath(sessionID), nil)
	require.NoError(t, err, "no se pudo armar la solicitud del canal")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	// Si las cabeceras no llegan en el plazo, se cancela la petición para no colgar la prueba.
	timer := time.AfterFunc(pokerStreamDeadline, cancel)
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if !timer.Stop() && err == nil {
		resp.Body.Close()
		err = context.DeadlineExceeded
	}
	if err != nil {
		cancel()
		require.NoError(t, err, "el canal de eventos no respondió en %s", pokerStreamDeadline)
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		cancel()
		require.Equal(t, http.StatusOK, resp.StatusCode, "el canal de eventos no se abrió; cuerpo: %s", data)
	}
	require.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"),
		"Content-Type = %q, se esperaba text/event-stream", resp.Header.Get("Content-Type"))

	stream := &pokerStream{
		env:       e,
		cancel:    cancel,
		body:      resp.Body,
		events:    make(chan pokerStreamEvent, 64),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		localAddr: localAddr,
	}
	go stream.read()
	t.Cleanup(stream.Close)
	return stream
}

// read interpreta el cuerpo del canal según text/event-stream: acumula las líneas event: y data:,
// ignora los comentarios que empiezan con ":" y entrega el evento al llegar la línea en blanco.
// Termina al cerrarse el cuerpo o al pedirse el cierre del lector.
func (s *pokerStream) read() {
	defer close(s.done)
	defer close(s.events)
	scanner := bufio.NewScanner(s.body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var name string
	var data []string
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		switch {
		case line == "":
			if name == "" && len(data) == 0 {
				continue
			}
			if name == "" {
				name = "message"
			}
			select {
			case s.events <- pokerStreamEvent{Name: name, Data: []byte(strings.Join(data, "\n"))}:
			case <-s.stop:
				return
			}
			name, data = "", nil
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
}

// Next devuelve la próxima instantánea (evento state) que llega por el canal ya abierto, sin
// recargar, y falla si no llega dentro del plazo de 2 segundos, si llega otro evento o si el canal
// se cierra.
func (s *pokerStream) Next(t *testing.T) pokerSessionBody {
	t.Helper()
	return s.nextBefore(t, time.Now().Add(pokerStreamDeadline))
}

// NextUntil lee instantáneas del canal hasta la primera que cumple la condición y falla si no llega
// ninguna que la cumpla dentro del plazo de 2 segundos desde la llamada. Sirve cuando un hecho
// anterior todavía puede tener su instantánea en camino.
func (s *pokerStream) NextUntil(t *testing.T, what string, cond func(pokerSessionBody) bool) pokerSessionBody {
	t.Helper()
	deadline := time.Now().Add(pokerStreamDeadline)
	var last *pokerSessionBody
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if last != nil {
				t.Fatalf("no llegó en %s una instantánea con %s; la última recibida: %s", pokerStreamDeadline, what, last.Raw)
			}
			t.Fatalf("no llegó en %s una instantánea con %s", pokerStreamDeadline, what)
		}
		snapshot := s.nextBefore(t, deadline)
		if cond(snapshot) {
			return snapshot
		}
		last = &snapshot
	}
}

// nextBefore espera el próximo evento hasta el instante límite y lo decodifica como instantánea.
func (s *pokerStream) nextBefore(t *testing.T, deadline time.Time) pokerSessionBody {
	t.Helper()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case ev, ok := <-s.events:
		if !ok {
			t.Fatalf("el canal de eventos se cerró sin enviar la instantánea esperada")
		}
		if ev.Name != "state" {
			t.Fatalf("se esperaba un evento state y llegó %q con datos %s", ev.Name, ev.Data)
		}
		var snapshot pokerSessionBody
		if err := json.Unmarshal(ev.Data, &snapshot); err != nil {
			t.Fatalf("los datos del evento state no son JSON válido (%v): %s", err, ev.Data)
		}
		snapshot.Raw = ev.Data
		return snapshot
	case <-timer.C:
		t.Fatalf("no llegó ninguna instantánea por el canal en %s", pokerStreamDeadline)
	}
	return pokerSessionBody{}
}

// Close corta la conexión del canal, espera a que termine el lector y, con el mismo plazo de 2
// segundos, a que el servidor termine de atender la conexión. Puede llamarse más de una vez.
func (s *pokerStream) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		s.cancel()
		s.body.Close()
		<-s.done
		if s.localAddr == "" {
			return
		}
		select {
		case <-s.env.connChannel(s.localAddr):
		case <-time.After(pokerStreamDeadline):
		}
	})
}
