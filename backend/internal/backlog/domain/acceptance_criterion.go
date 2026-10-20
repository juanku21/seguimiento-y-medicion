package domain

import (
	"time"

	"github.com/google/uuid"
)

// AcceptanceCriterion es un enunciado que define cuándo una historia se considera terminada; se
// persiste en la tabla acceptance_criteria (data-model.md sección 2). El ID lo genera la aplicación
// con google/uuid. Position empieza en 0 y es consecutiva dentro de la historia; el índice único
// (story_id, position) garantiza un solo criterio por posición y sirve a la lectura ordenada y al
// conteo del listado. Text guarda el enunciado ya recortado, de hasta 500 runas. El campo Story
// existe solo para que AutoMigrate cree la clave foránea hacia user_stories.id: no es columna y
// nunca se precarga. La FK no lleva ON DELETE CASCADE porque el repositorio borra los criterios de
// forma explícita (research.md sección 12), y CreatedAt y UpdatedAt los completa GORM.
type AcceptanceCriterion struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	StoryID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_acceptance_criteria_story_position,priority:1"`
	Story     *UserStory `gorm:"foreignKey:StoryID;references:ID"`
	Position  int        `gorm:"type:integer;not null;uniqueIndex:idx_acceptance_criteria_story_position,priority:2"`
	Text      string     `gorm:"type:varchar(500);not null"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt time.Time  `gorm:"type:timestamptz;not null"`
}

// TableName fija el nombre de la tabla en acceptance_criteria, porque la pluralización por defecto de
// GORM produciría acceptance_criterions (data-model.md sección 2).
func (AcceptanceCriterion) TableName() string {
	return "acceptance_criteria"
}
