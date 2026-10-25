// Package domain declara las entidades, los errores, las reglas puras y las interfaces del módulo de
// registro de esfuerzo; no depende de ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EffortEntry son las horas que una persona trabajó sobre una historia en un día concreto y qué hizo;
// se persiste en la tabla effort_entries (data-model.md sección 1). El ID lo genera la aplicación con
// google/uuid. StoryID y UserID son inmutables: UserID es siempre el titular de la sesión que creó el
// registro. SprintID es anulable (NULL significa sin sprint), se fija en el alta y nunca cambia.
// EntryDate es una columna date que se maneja a medianoche UTC. Activity se guarda recortada, de 1 a
// 300 caracteres. HoursHundredths son centésimas de hora: 25 a 2400, múltiplo de 25. El índice
// (story_id, entry_date, created_at) sirve al esfuerzo de una historia y a su orden; el índice
// (user_id, entry_date) sirve a la suma del tope diario y al filtro por integrante; el índice sobre
// sprint_id sirve a SprintEffort.HasEffort y al filtro por sprint. Los campos Story, User y Sprint
// existen solo para que AutoMigrate cree las claves foráneas hacia user_stories.id, users.id y
// sprints.id, sin ON DELETE CASCADE: no son columnas y nunca se precargan. No hay columnas de
// proyecto (se deriva de la historia), de borrado lógico ni de "definitivo" (se deriva del sprint),
// y CreatedAt y UpdatedAt los completa GORM.
type EffortEntry struct {
	ID              uuid.UUID                `gorm:"type:uuid;primaryKey"`
	StoryID         uuid.UUID                `gorm:"type:uuid;not null;index:idx_effort_entries_story_order,priority:1"`
	Story           *backlogdomain.UserStory `gorm:"foreignKey:StoryID;references:ID"`
	UserID          uuid.UUID                `gorm:"type:uuid;not null;index:idx_effort_entries_user_date,priority:1"`
	User            *authdomain.User         `gorm:"foreignKey:UserID;references:ID"`
	SprintID        *uuid.UUID               `gorm:"type:uuid;index:idx_effort_entries_sprint"`
	Sprint          *sprintsdomain.Sprint    `gorm:"foreignKey:SprintID;references:ID"`
	EntryDate       time.Time                `gorm:"type:date;not null;index:idx_effort_entries_story_order,priority:2;index:idx_effort_entries_user_date,priority:2"`
	Activity        string                   `gorm:"type:varchar(300);not null"`
	HoursHundredths int                      `gorm:"type:integer;not null"`
	CreatedAt       time.Time                `gorm:"type:timestamptz;not null;index:idx_effort_entries_story_order,priority:3"`
	UpdatedAt       time.Time                `gorm:"type:timestamptz;not null"`
}
