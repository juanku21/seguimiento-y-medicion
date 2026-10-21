package domain

import (
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// SprintStory es el compromiso de una historia en un sprint; se persiste en la tabla sprint_stories
// (data-model.md sección 2). Mientras el sprint está abierto es el plan vivo: guarda el momento del
// compromiso y, si la historia está Completada, CompletedAt. Al cerrar el sprint se completan una
// sola vez TitleAtClose, PriorityAtClose, StoryPointsAtClose y Outcome, y la fila pasa a formar parte
// de la instantánea; por eso esos campos son anulables y valen NULL mientras el sprint está abierto
// (research.md sección 6). El índice único (sprint_id, story_id) impide repetir una historia en un
// mismo sprint; el índice único parcial sobre story_id con outcome IS NULL respalda en la base que una
// historia tenga como máximo un compromiso abierto (FR-020), y el índice (story_id, committed_at)
// sirve al historial de la historia (FR-055). Los campos Sprint y Story existen solo para que
// AutoMigrate cree las claves foráneas hacia sprints.id y user_stories.id, sin ON DELETE CASCADE: no
// son columnas y nunca se precargan. CreatedAt y UpdatedAt los completa GORM.
type SprintStory struct {
	ID                 uuid.UUID                    `gorm:"type:uuid;primaryKey"`
	SprintID           uuid.UUID                    `gorm:"type:uuid;not null;uniqueIndex:idx_sprint_stories_sprint_story,priority:1"`
	Sprint             *Sprint                      `gorm:"foreignKey:SprintID;references:ID"`
	StoryID            uuid.UUID                    `gorm:"type:uuid;not null;uniqueIndex:idx_sprint_stories_sprint_story,priority:2;uniqueIndex:idx_sprint_stories_one_open_per_story,where:outcome IS NULL;index:idx_sprint_stories_story,priority:1"`
	Story              *backlogdomain.UserStory     `gorm:"foreignKey:StoryID;references:ID"`
	CommittedAt        time.Time                    `gorm:"type:timestamptz;not null;index:idx_sprint_stories_story,priority:2"`
	CompletedAt        *time.Time                   `gorm:"type:timestamptz"`
	TitleAtClose       *string                      `gorm:"type:varchar(150)"`
	PriorityAtClose    *backlogdomain.StoryPriority `gorm:"type:varchar(8)"`
	StoryPointsAtClose *int                         `gorm:"type:smallint"`
	Outcome            *StoryOutcome                `gorm:"type:varchar(16)"`
	CreatedAt          time.Time                    `gorm:"type:timestamptz;not null"`
	UpdatedAt          time.Time                    `gorm:"type:timestamptz;not null"`
}

// StoryOutcome es el resultado de una historia en un sprint; se guarda como texto en la columna
// sprint_stories.outcome y no como enum de PostgreSQL (data-model.md sección 2). Queda NULL mientras
// el sprint está abierto y se completa una sola vez, al cerrarlo (FR-042, FR-043).
type StoryOutcome string

// Resultados posibles al cerrar el sprint: completed si la historia estaba Completada y
// not_completed en cualquier otro caso (FR-043).
const (
	StoryOutcomeCompleted    StoryOutcome = "completed"
	StoryOutcomeNotCompleted StoryOutcome = "not_completed"
)
