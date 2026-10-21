// Package domain declara las entidades, los errores y las interfaces del módulo de sprints; no
// depende de ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Sprint es el período acotado de trabajo dentro de un proyecto, con un objetivo y un conjunto de
// historias comprometidas; se persiste en la tabla sprints (data-model.md sección 1). El ID lo
// genera la aplicación con google/uuid. Name no tiene índice único porque dos sprints del mismo
// proyecto pueden llamarse igual. StartDate y EndDate son columnas date que se manejan a medianoche
// UTC, igual que las fechas del proyecto. Status usa el tipo SprintStatus, que se guarda como texto
// (planned, active, closed) y no como enum de PostgreSQL. CommittedStoryPoints y StartedAt se fijan
// al iniciar; ClosedAt, ClosedByID y HoursPerStoryPointHundredths (factor del proyecto copiado al
// cerrar, en centésimas de hora) se fijan al cerrar, y por eso todos son anulables. El índice
// compuesto (project_id, start_date, id) sirve al listado, a su orden y a la verificación de
// superposición; el índice único parcial sobre project_id con status = 'active' respalda en la base
// la regla de un solo sprint activo por proyecto. Los campos Project, ClosedBy y CreatedBy existen
// solo para que AutoMigrate cree las claves foráneas hacia projects.id y users.id: no son columnas y
// nunca se precargan. No hay columnas de totales planificados ni completados porque se derivan
// (research.md sección 11), y CreatedAt y UpdatedAt los completa GORM.
type Sprint struct {
	ID                           uuid.UUID               `gorm:"type:uuid;primaryKey;index:idx_sprints_project_start,priority:3"`
	ProjectID                    uuid.UUID               `gorm:"type:uuid;not null;index:idx_sprints_project_start,priority:1;uniqueIndex:idx_sprints_one_active_per_project,where:status = 'active'"`
	Project                      *projectsdomain.Project `gorm:"foreignKey:ProjectID;references:ID"`
	Name                         string                  `gorm:"type:varchar(100);not null"`
	Goal                         string                  `gorm:"type:varchar(500);not null"`
	StartDate                    time.Time               `gorm:"type:date;not null;index:idx_sprints_project_start,priority:2"`
	EndDate                      time.Time               `gorm:"type:date;not null"`
	Status                       SprintStatus            `gorm:"type:varchar(16);not null"`
	CommittedStoryPoints         *int                    `gorm:"type:integer"`
	StartedAt                    *time.Time              `gorm:"type:timestamptz"`
	ClosedAt                     *time.Time              `gorm:"type:timestamptz"`
	ClosedByID                   *uuid.UUID              `gorm:"type:uuid"`
	ClosedBy                     *authdomain.User        `gorm:"foreignKey:ClosedByID;references:ID"`
	HoursPerStoryPointHundredths *int                    `gorm:"type:integer"`
	CreatedByID                  uuid.UUID               `gorm:"type:uuid;not null"`
	CreatedBy                    *authdomain.User        `gorm:"foreignKey:CreatedByID;references:ID"`
	CreatedAt                    time.Time               `gorm:"type:timestamptz;not null"`
	UpdatedAt                    time.Time               `gorm:"type:timestamptz;not null"`
}
