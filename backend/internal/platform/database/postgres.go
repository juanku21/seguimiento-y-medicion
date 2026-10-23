// Package database abre la conexión GORM con PostgreSQL y crea o actualiza el esquema con
// AutoMigrate, sin herramientas de migración externas (research.md sección 7).
package database

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Open se conecta a la base indicada por la configuración con TranslateError activado, para que
// las violaciones de restricciones lleguen como errores de GORM (por ejemplo gorm.ErrDuplicatedKey)
// y no como códigos propios de PostgreSQL (research.md sección 3), y migra los modelos de las
// features de autenticación, de proyectos, del backlog, de sprints y de Planning Poker. Lo usan cmd/app al arrancar y el ayudante de las pruebas de integración.
func Open(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(DSN(cfg)), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con PostgreSQL en %s:%s: %w", cfg.DBHost, cfg.DBPort, err)
	}

	// El orden respeta las claves foráneas: User va primero porque sessions, auth_events, projects,
	// project_members, user_stories y sprints apuntan a users.id; Project va antes que ProjectMember,
	// UserStory y Sprint porque project_members, user_stories y sprints apuntan a projects.id;
	// UserStory va antes que AcceptanceCriterion porque acceptance_criteria apunta a user_stories.id,
	// y Sprint y SprintStory siguen porque sprint_stories apunta a sprints.id y a user_stories.id.
	// Las entidades de Planning Poker van al final, en cadena: EstimationSession apunta a
	// user_stories.id y a users.id, EstimationRound y EstimationParticipant a estimation_sessions.id,
	// y EstimationVote a estimation_rounds.id y a estimation_participants.id. Migrar UserStory
	// agrega además la columna ever_committed. Los índices
	// compuestos, únicos o no, incluidos los parciales con cláusula WHERE, los crean las etiquetas de
	// GORM de cada modelo. Si la migración falla se cierra la conexión para no dejar el pool abierto.
	if err := db.AutoMigrate(
		&authdomain.User{},
		&authdomain.Session{},
		&authdomain.AuthEvent{},
		&projectsdomain.Project{},
		&projectsdomain.ProjectMember{},
		&backlogdomain.UserStory{},
		&backlogdomain.AcceptanceCriterion{},
		&sprintsdomain.Sprint{},
		&sprintsdomain.SprintStory{},
		&pokerdomain.EstimationSession{},
		&pokerdomain.EstimationRound{},
		&pokerdomain.EstimationParticipant{},
		&pokerdomain.EstimationVote{},
	); err != nil {
		migrateErr := fmt.Errorf("no se pudo migrar el esquema: %w", err)
		sqlDB, dbErr := db.DB()
		if dbErr != nil {
			return nil, errors.Join(migrateErr, dbErr)
		}
		return nil, errors.Join(migrateErr, sqlDB.Close())
	}
	return db, nil
}

// DSN arma la URL de conexión con net/url para que una contraseña con caracteres especiales quede
// escapada. El SSL se desactiva porque la base corre en un contenedor local de Docker Compose. Se
// exporta para que el ayudante de pruebas abra su conexión dedicada al lock con la misma URL.
func DSN(cfg config.Config) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:     net.JoinHostPort(cfg.DBHost, cfg.DBPort),
		Path:     cfg.DBName,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
