// Package main compone las dependencias del backend y arranca el servidor HTTP; no contiene lógica
// de negocio (Principio III).
//
//	@title						Software Metrics & Estimation — API
//	@version					1.0.0
//	@description				API de Software Metrics & Estimation: autenticación y cuentas de usuario, gestión de
//	@description				proyectos y de sus integrantes, Product Backlog de cada proyecto, planificación de sus
//	@description				sprints, estimación de historias con Planning Poker, registro del esfuerzo trabajado
//	@description				sobre cada historia, gestión de los defectos de cada proyecto y cálculo de las métricas
//	@description				de estimación, seguimiento y calidad de cada sprint.
//	@description				Las únicas operaciones públicas son el registro, el inicio de sesión y el estado del servicio; las demás exigen
//	@description				la cabecera `Authorization: Bearer <token>`.
//	@BasePath					/
//	@schemes					http
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Token emitido por /api/v1/auth/login, enviado como `Bearer <token>`.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
	// time/tzdata embute la base de zonas horarias en el binario: la imagen alpine del backend no
	// la trae y sin ella time.LoadLocation fallaría solo dentro del contenedor.
	_ "time/tzdata"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/juanku21/seguimiento-y-medicion/backend/docs"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	backlogdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/delivery"
	backlogrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	backlogservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
	defectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/delivery"
	defectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/repository"
	defectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/service"
	effortdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/delivery"
	effortrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/repository"
	effortservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/service"
	metricsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/delivery"
	metricsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/repository"
	metricsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/database"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	pokerdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/delivery"
	pokerrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/repository"
	pokerservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/service"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
	sprintsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/delivery"
	sprintsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
	sprintsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// envFile es el archivo de variables de desarrollo, relativo al directorio backend/ desde el que
// se ejecuta `go run ./cmd/app`.
const envFile = ".env"

// main informa cualquier error de arranque y termina con código distinto de cero.
func main() {
	if err := run(); err != nil {
		slog.Error("el servidor no pudo arrancar", "error", err)
		os.Exit(1)
	}
}

// run carga la configuración, abre la base, arma el router y queda escuchando hasta que el
// servidor se detiene.
func run() error {
	// El archivo .env es opcional: en un contenedor las variables llegan ya cargadas en el entorno.
	if err := config.LoadFile(envFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// La conexión migra el esquema al abrirse; se cierra el pool al terminar el proceso.
	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("no se pudo obtener el pool de conexiones: %w", err)
	}
	defer sqlDB.Close()

	// Se componen los repositorios de cuentas, sesiones y eventos, el servicio y los manejadores de
	// la feature de autenticación.
	auth := service.NewAuthService(
		repository.NewUserRepository(db),
		repository.NewSessionRepository(db),
		repository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	handler := delivery.NewHandler(auth)

	// El router habilita CORS solo para el origen del frontend, limita el cuerpo de cada solicitud a
	// 1 MiB, rechaza con 400 las queries mal formadas para que ningún listado ignore sus filtros sin
	// aviso, publica Swagger y el endpoint de salud, y registra las rutas de la feature de
	// autenticación. La ruta de Swagger es pública sin sesión: es la excepción a RN-08 documentada en
	// specs/001-user-auth/spec.md, porque el Principio VIII exige documentar la API con Swagger.
	// GET /healthz también es público, porque lo consulta el health check de Render sin token, y
	// solo informa si la base responde.
	router := gin.Default()
	router.Use(httpserver.CORS(cfg.CORSAllowedOrigin))
	router.Use(httpserver.LimitRequestBody())
	router.Use(httpserver.RejectMalformedQuery())
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/healthz", httpserver.Health(sqlDB))
	delivery.RegisterRoutes(router, handler)

	// El módulo de proyectos recibe el middleware de sesión de la feature de autenticación como
	// gin.HandlerFunc, así todo su grupo de rutas exige sesión sin importar ese middleware
	// (FR-046, research.md de la 002, sección 10). Sus manejadores se componen sobre el servicio de
	// proyectos, con el repositorio de proyectos y los puertos de backlog y de sprints, y sobre el
	// servicio de integrantes, con el directorio de usuarios, y el de cambio de estado, con el
	// puerto de sprints, la zona horaria del sistema y el reloj. Las mismas instancias de los
	// repositorios sirven a los servicios y a RequireMembership del subgrupo /:projectId.
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
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, handler.RequireSession, projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// El módulo del backlog se compone sobre el repositorio de historias y los cinco puertos hacia
	// otras features: los defectos, que leen la marca ever_had_defects de la 007, y las sesiones de
	// estimación, que todavía no existen y responden sin historial; el esfuerzo, que lee la marca ever_had_effort que deja el alta de la 006; las
	// asignaciones a sprints, que leen los compromisos de la 004, y el directorio de proyectos, que
	// lee el estado de la 002. Sus rutas
	// cuelgan del subgrupo /api/v1/projects/:projectId, que ya exige sesión y membresía, recibido
	// aquí para que backlog/delivery no importe projects/delivery. Las rutas de ítem van en el grupo
	// plano /api/v1/stories/:storyId, que exige sesión y RequireStoryAccess sobre la misma instancia
	// del repositorio de historias que usa el servicio.
	storyRepository := backlogrepository.NewStoryRepository(db)
	stories := backlogservice.NewStoryService(
		storyRepository,
		backlogrepository.NewSprintAssignments(db),
		backlogrepository.NewEffortRecords(db),
		backlogrepository.NewDefectRecords(db),
		backlogrepository.NewEstimationSessions(db),
		backlogrepository.NewProjectDirectory(db),
	)
	backlogHandler := backlogdelivery.NewHandler(stories)
	backlogdelivery.RegisterRoutes(projectGroup, backlogHandler)
	backlogdelivery.RegisterStoryRoutes(router, handler.RequireSession, storyRepository, backlogHandler)

	// El módulo de sprints se compone sobre su repositorio y el puerto de esfuerzo, que lee los
	// registros de la 006 anclados al sprint, con la zona horaria del sistema y el reloj. Sus rutas de
	// colección cuelgan del subgrupo /api/v1/projects/:projectId, que ya exige sesión y membresía,
	// recibido aquí para que sprints/delivery no importe projects/delivery. Las rutas de ítem van en el
	// grupo plano /api/v1/sprints/:sprintId, que exige sesión y RequireSprintAccess sobre la misma
	// instancia del repositorio de sprints que usa el servicio. Las rutas de sprints sobre una historia
	// reciben como gin.HandlerFunc el RequireStoryAccess de la 003, sobre la misma instancia del
	// repositorio de historias, para que sprints/delivery no importe backlog/delivery.
	sprintRepository := sprintsrepository.NewSprintRepository(db)
	sprints := sprintsservice.NewSprintService(
		sprintRepository,
		sprintsrepository.NewSprintEffort(db),
		cfg.Location,
		time.Now,
	)
	sprintsHandler := sprintsdelivery.NewHandler(sprints)
	sprintsdelivery.RegisterRoutes(projectGroup, sprintsHandler)
	sprintsdelivery.RegisterSprintRoutes(router, handler.RequireSession, sprintRepository, sprintsHandler)
	sprintsdelivery.RegisterStoryRoutes(router, handler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), sprintsHandler)

	// El módulo de Planning Poker se compone sobre su repositorio y el reloj del sistema. La apertura
	// cuelga del grupo plano /api/v1/stories/:storyId con el RequireStoryAccess de la 003, sobre la
	// misma instancia del repositorio de historias, para que poker/delivery no importe
	// backlog/delivery; el resto va en el grupo plano /api/v1/estimation-sessions/:sessionId, con
	// RequireEstimationAccess sobre la misma instancia del repositorio de sesiones que usa el servicio.
	sessionRepository := pokerrepository.NewSessionRepository(db)
	sessions := pokerservice.NewSessionService(sessionRepository, time.Now)
	pokerHandler := pokerdelivery.NewHandler(sessions)
	pokerdelivery.RegisterStoryRoutes(router, handler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), pokerHandler)
	pokerdelivery.RegisterSessionRoutes(router, handler.RequireSession, sessionRepository, pokerHandler)

	// El módulo de esfuerzo se compone sobre su repositorio, con la zona horaria del sistema, el plazo
	// de gracia configurado y el reloj. Sus rutas cuelgan del grupo plano /api/v1/stories/:storyId con
	// el RequireStoryAccess de la 003, sobre la misma instancia del repositorio de historias, para que
	// effort/delivery no importe backlog/delivery. La corrección y la eliminación de un registro
	// cuelgan del grupo plano /api/v1/effort-entries/:entryId, con RequireEffortEntryAccess sobre la
	// misma instancia del repositorio de esfuerzo que usa el servicio. La consulta del esfuerzo del
	// proyecto cuelga del subgrupo /:projectId de la 002, que ya exige sesión y membresía vigente.
	effortRepository := effortrepository.NewEffortRepository(db)
	efforts := effortservice.NewEffortService(
		effortRepository,
		cfg.Location,
		cfg.EffortGracePeriod,
		time.Now,
	)
	effortHandler := effortdelivery.NewHandler(efforts)
	effortdelivery.RegisterStoryRoutes(router, handler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), effortHandler)
	effortdelivery.RegisterEntryRoutes(router, handler.RequireSession, effortRepository, effortHandler)
	effortdelivery.RegisterProjectRoutes(projectGroup, effortHandler)

	// El módulo de defectos se compone sobre su repositorio y el reloj del sistema. El alta cuelga del
	// subgrupo /:projectId de la 002, que ya exige sesión y membresía vigente, recibido aquí para que
	// defects/delivery no importe projects/delivery; el detalle y el cambio de estado van en el grupo
	// plano /api/v1/defects/:defectId, con la misma instancia del repositorio para RequireDefectAccess.
	defectRepository := defectsrepository.NewDefectRepository(db)
	defects := defectsservice.NewDefectService(defectRepository, time.Now)
	defectHandler := defectsdelivery.NewHandler(defects)
	defectsdelivery.RegisterProjectRoutes(projectGroup, defectHandler)
	defectsdelivery.RegisterDefectRoutes(router, handler.RequireSession, defectRepository, defectHandler)
	defectsdelivery.RegisterStoryRoutes(router, handler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), defectHandler)

	// El módulo de métricas se compone sobre su repositorio, con el plazo de gracia configurado y el
	// reloj del sistema. Sus rutas cuelgan del subgrupo /:projectId de la 002, que ya exige sesión y
	// membresía vigente, recibido aquí para que metrics/delivery no importe projects/delivery.
	metricsCalculator := metricsservice.NewMetricsService(metricsrepository.NewMetricsRepository(db), cfg.EffortGracePeriod, time.Now)
	metricsdelivery.RegisterRoutes(projectGroup, metricsdelivery.NewHandler(metricsCalculator))

	// Ninguna conexión al canal de eventos sobrevive a un reinicio, así que antes de escuchar se
	// marcan desconectados los participantes de las sesiones abiertas (research.md sección 9). El
	// barrido periódico corre en su goroutine cada segundo hasta que run termina (sección 13).
	if err := sessionRepository.ResetPresence(context.Background(), time.Now()); err != nil {
		return err
	}
	sweepCtx, stopSweep := context.WithCancel(context.Background())
	defer stopSweep()
	go sessions.Run(sweepCtx, time.Second)

	// ReadHeaderTimeout acota el tiempo para recibir las cabeceras y ReadTimeout el de la solicitud
	// completa, cuerpo incluido, para evitar conexiones colgadas.
	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
	}
	slog.Info("servidor escuchando", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("el servidor HTTP se detuvo: %w", err)
	}
	return nil
}
