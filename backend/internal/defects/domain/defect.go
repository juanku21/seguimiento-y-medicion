// Package domain declara las entidades, los errores, las reglas puras y las interfaces del módulo de
// seguimiento de defectos; no depende de ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Defect es un problema detectado en el producto, con sus dos anclas de sprint; se persiste en la
// tabla defects (data-model.md sección 1). El ID lo genera la aplicación con google/uuid. Description
// se guarda recortada, de 1 a 2000 caracteres. Severity y Status toman los valores de DefectSeverity
// y DefectStatus. DetectedSprintID es el sprint donde se detectó; ResolvedSprintID es anulable y es
// no nulo si y solo si Status es resolved. CreatedByID es inmutable, UpdatedByID es el titular de la
// última escritura y StatusChangedByID y StatusChangedAt corresponden al último cambio de estado (en
// el alta, a la creación). El índice (story_id, created_at) sirve a los defectos de una historia; los
// índices sobre detected_sprint_id y resolved_sprint_id sirven al filtro y al conteo por sprint. Los
// campos Story, DetectedSprint, ResolvedSprint, CreatedBy, UpdatedBy y StatusChangedBy existen solo
// para que AutoMigrate cree las claves foráneas, sin ON DELETE CASCADE (la de Story se llama
// fk_defects_story): no son columnas y nunca se precargan. No hay columnas de proyecto (se deriva de
// la historia), de borrado lógico ni de historial, y CreatedAt y UpdatedAt los completa GORM.
type Defect struct {
	ID                uuid.UUID                `gorm:"type:uuid;primaryKey"`
	StoryID           uuid.UUID                `gorm:"type:uuid;not null;index:idx_defects_story_order,priority:1"`
	Story             *backlogdomain.UserStory `gorm:"foreignKey:StoryID;references:ID"`
	Description       string                   `gorm:"type:varchar(2000);not null"`
	Severity          DefectSeverity           `gorm:"type:varchar(8);not null"`
	Status            DefectStatus             `gorm:"type:varchar(16);not null"`
	DetectedSprintID  uuid.UUID                `gorm:"type:uuid;not null;index:idx_defects_detected_sprint"`
	DetectedSprint    *sprintsdomain.Sprint    `gorm:"foreignKey:DetectedSprintID;references:ID"`
	ResolvedSprintID  *uuid.UUID               `gorm:"type:uuid;index:idx_defects_resolved_sprint"`
	ResolvedSprint    *sprintsdomain.Sprint    `gorm:"foreignKey:ResolvedSprintID;references:ID"`
	CreatedByID       uuid.UUID                `gorm:"type:uuid;not null"`
	CreatedBy         *authdomain.User         `gorm:"foreignKey:CreatedByID;references:ID"`
	UpdatedByID       uuid.UUID                `gorm:"type:uuid;not null"`
	UpdatedBy         *authdomain.User         `gorm:"foreignKey:UpdatedByID;references:ID"`
	StatusChangedByID uuid.UUID                `gorm:"type:uuid;not null"`
	StatusChangedBy   *authdomain.User         `gorm:"foreignKey:StatusChangedByID;references:ID"`
	StatusChangedAt   time.Time                `gorm:"type:timestamptz;not null"`
	CreatedAt         time.Time                `gorm:"type:timestamptz;not null;index:idx_defects_story_order,priority:2"`
	UpdatedAt         time.Time                `gorm:"type:timestamptz;not null"`
}
