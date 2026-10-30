package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// ReportGeneration es el registro de auditoría de cada reporte entregado: quién lo generó, cuándo y
// con qué alcance; se persiste en la tabla report_generations (data-model.md sección 1). SprintID es
// nulo en el alcance project. Los campos Project, Sprint y GeneratedBy existen solo para que
// AutoMigrate cree las claves foráneas, sin cascada: no son columnas y nunca se precargan. El
// registro nunca se actualiza ni se elimina.
type ReportGeneration struct {
	ID            uuid.UUID               `gorm:"type:uuid;primaryKey"`
	ProjectID     uuid.UUID               `gorm:"type:uuid;not null;index:idx_report_generations_project_generated,priority:1"`
	Project       *projectsdomain.Project `gorm:"foreignKey:ProjectID;references:ID"`
	SprintID      *uuid.UUID              `gorm:"type:uuid"`
	Sprint        *sprintsdomain.Sprint   `gorm:"foreignKey:SprintID;references:ID"`
	Scope         ReportScope             `gorm:"type:varchar(8);not null"`
	GeneratedByID uuid.UUID               `gorm:"type:uuid;not null"`
	GeneratedBy   *authdomain.User        `gorm:"foreignKey:GeneratedByID;references:ID"`
	GeneratedAt   time.Time               `gorm:"type:timestamptz;not null;index:idx_report_generations_project_generated,priority:2"`
	CreatedAt     time.Time               `gorm:"type:timestamptz;not null"`
	UpdatedAt     time.Time               `gorm:"type:timestamptz;not null"`
}
