// Package main compone las dependencias del backend y arranca el servidor HTTP; no contiene lógica
// de negocio (Principio III).
//
//	@title						Software Metrics & Estimation — API
//	@version					1.0.0
//	@description				API de autenticación y cuentas de usuario de Software Metrics & Estimation.
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

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/juanku21/seguimiento-y-medicion/backend/docs"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/database"
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

	// El router habilita CORS solo para el origen del frontend, publica Swagger y registra las
	// rutas de la feature de autenticación.
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{cfg.CORSAllowedOrigin},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		MaxAge:       12 * time.Hour,
	}))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	delivery.RegisterRoutes(router)

	// ReadHeaderTimeout acota el tiempo para recibir las cabeceras y evita conexiones colgadas.
	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("servidor escuchando", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("el servidor HTTP se detuvo: %w", err)
	}
	return nil
}
