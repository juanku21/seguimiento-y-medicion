// Package main compone las dependencias del backend y arranca el servidor HTTP; no contiene lógica
// de negocio (Principio III).
//
//	@title						Software Metrics & Estimation — API
//	@version					1.0.0
//	@description				API de Software Metrics & Estimation: autenticación y cuentas de usuario, gestión de
//	@description				proyectos y de sus integrantes, Product Backlog de cada proyecto y planificación de sus
//	@description				sprints.
//	@description				Las únicas operaciones públicas son el registro y el inicio de sesión; las demás exigen
//	@description				la cabecera `Authorization: Bearer <token>`.
//	@BasePath					/
//	@schemes					http
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Token emitido por /api/v1/auth/login, enviado como `Bearer <token>`.
package main

import (
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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/database"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
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
	// 1 MiB, publica Swagger y registra las rutas de la feature de autenticación. La ruta de Swagger
	// es pública sin sesión: es la excepción a RN-08 documentada en specs/001-user-auth/spec.md,
	// porque el Principio VIII exige documentar la API con Swagger.
	router := gin.Default()
	router.Use(httpserver.CORS(cfg.CORSAllowedOrigin))
	router.Use(httpserver.LimitRequestBody())
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
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
		projectsrepository.NewSprintDirectory(),
	)
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(), cfg.Location, time.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, handler.RequireSession, projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// El módulo del backlog se compone sobre el repositorio de historias y los cinco puertos hacia
	// otras features: sprints, esfuerzo, defectos y sesiones de estimación, que todavía no existen y
	// responden sin historial, y el directorio de proyectos, que lee el estado de la 002. Sus rutas
	// cuelgan del subgrupo /api/v1/projects/:projectId, que ya exige sesión y membresía, recibido
	// aquí para que backlog/delivery no importe projects/delivery. Las rutas de ítem van en el grupo
	// plano /api/v1/stories/:storyId, que exige sesión y RequireStoryAccess sobre la misma instancia
	// del repositorio de historias que usa el servicio.
	storyRepository := backlogrepository.NewStoryRepository(db)
	stories := backlogservice.NewStoryService(
		storyRepository,
		backlogrepository.NewSprintAssignments(),
		backlogrepository.NewEffortRecords(),
		backlogrepository.NewDefectRecords(),
		backlogrepository.NewEstimationSessions(),
		backlogrepository.NewProjectDirectory(db),
	)
	backlogHandler := backlogdelivery.NewHandler(stories)
	backlogdelivery.RegisterRoutes(projectGroup, backlogHandler)
	backlogdelivery.RegisterStoryRoutes(router, handler.RequireSession, storyRepository, backlogHandler)

	// El módulo de sprints se compone sobre su repositorio y el puerto de esfuerzo, que todavía no
	// existe y responde sin esfuerzo, con la zona horaria del sistema y el reloj. Sus rutas de
	// colección cuelgan del subgrupo /api/v1/projects/:projectId, que ya exige sesión y membresía,
	// recibido aquí para que sprints/delivery no importe projects/delivery.
	sprints := sprintsservice.NewSprintService(
		sprintsrepository.NewSprintRepository(db),
		sprintsrepository.NewSprintEffort(),
		cfg.Location,
		time.Now,
	)
	sprintsdelivery.RegisterRoutes(projectGroup, sprintsdelivery.NewHandler(sprints))

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
