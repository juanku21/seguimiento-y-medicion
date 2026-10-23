// Package domain declara las entidades, los errores y las interfaces del módulo de Planning Poker;
// no depende de ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// EstimationSession es el encuentro de Planning Poker sobre una única historia, contenedor de sus
// rondas, participantes y votos; se persiste en la tabla estimation_sessions (data-model.md sección
// 1). El ID lo genera la aplicación con google/uuid y StoryID no cambia nunca. Status es un
// SessionStatus (open, finished, canceled) guardado como texto y no como enum de PostgreSQL; la
// sesión nace open. Los campos anulables reflejan las invariantes por estado: AgreedStoryPoints solo
// se completa al finalizar, CancelReason (inactivity, story_completed, project_finished) solo en las
// cancelaciones del sistema, y ClosedByID y ClosedAt al cerrar, con ClosedByID nulo si cerró el
// sistema.
// LastActivityAt es el momento del último hecho y Version sube con cada hecho. CreatedAt es el
// momento de apertura. El índice único parcial sobre story_id con status = 'open' respalda en la
// base que una historia tenga como máximo una sesión abierta (RN-04), y el índice (story_id,
// created_at) sirve al historial de la historia (FR-080). Los campos Story, Facilitator, OpenedBy y
// ClosedBy existen solo para que AutoMigrate cree las claves foráneas hacia user_stories.id y
// users.id, sin ON DELETE: no son columnas y nunca se precargan. CreatedAt y UpdatedAt los completa
// GORM.
type EstimationSession struct {
	ID                uuid.UUID                `gorm:"type:uuid;primaryKey"`
	StoryID           uuid.UUID                `gorm:"type:uuid;not null;uniqueIndex:idx_estimation_sessions_one_open_per_story,where:status = 'open';index:idx_estimation_sessions_story_created,priority:1"`
	Story             *backlogdomain.UserStory `gorm:"foreignKey:StoryID;references:ID"`
	FacilitatorID     uuid.UUID                `gorm:"type:uuid;not null"`
	Facilitator       *authdomain.User         `gorm:"foreignKey:FacilitatorID;references:ID"`
	Status            SessionStatus            `gorm:"type:varchar(16);not null"`
	AgreedStoryPoints *int                     `gorm:"type:smallint"`
	CancelReason      *CancelReason            `gorm:"type:varchar(24)"`
	OpenedByID        uuid.UUID                `gorm:"type:uuid;not null"`
	OpenedBy          *authdomain.User         `gorm:"foreignKey:OpenedByID;references:ID"`
	ClosedByID        *uuid.UUID               `gorm:"type:uuid"`
	ClosedBy          *authdomain.User         `gorm:"foreignKey:ClosedByID;references:ID"`
	ClosedAt          *time.Time               `gorm:"type:timestamptz"`
	LastActivityAt    time.Time                `gorm:"type:timestamptz;not null"`
	Version           int64                    `gorm:"type:bigint;not null;default:0"`
	CreatedAt         time.Time                `gorm:"type:timestamptz;not null;index:idx_estimation_sessions_story_created,priority:2"`
	UpdatedAt         time.Time                `gorm:"type:timestamptz;not null"`
}
