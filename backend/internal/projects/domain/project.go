// Package domain declara las entidades, los errores y las interfaces del módulo de proyectos; no
// depende de ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// Project es el emprendimiento de software que el equipo estima, planifica, sigue y mide; se
// persiste en la tabla projects (data-model.md sección 1). El ID lo genera la aplicación con
// google/uuid. Name es el nombre tal como se escribió y NameNormalized su versión para comparar,
// única por propietario gracias al índice compuesto (owner_id, name_normalized). Las fechas son
// columnas date sin hora ni zona, y ActualEndDate solo existe cuando el proyecto está finalizado. El
// factor de conversión se guarda en centésimas de hora enteras, entre 1 y 4000, para no redondear.
// El campo Owner existe solo para que AutoMigrate cree la clave foránea de owner_id hacia users.id:
// no es una columna y nunca se precarga. El modelo no declara colecciones hacia project_members, y
// CreatedAt y UpdatedAt los completa GORM.
type Project struct {
	ID                           uuid.UUID        `gorm:"type:uuid;primaryKey"`
	OwnerID                      uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_projects_owner_name_normalized,priority:1"`
	Owner                        *authdomain.User `gorm:"foreignKey:OwnerID;references:ID"`
	Name                         string           `gorm:"type:varchar(100);not null"`
	NameNormalized               string           `gorm:"type:varchar(100);not null;uniqueIndex:idx_projects_owner_name_normalized,priority:2"`
	Description                  *string          `gorm:"type:varchar(1000)"`
	StartDate                    time.Time        `gorm:"type:date;not null"`
	PlannedEndDate               time.Time        `gorm:"type:date;not null"`
	ActualEndDate                *time.Time       `gorm:"type:date"`
	HoursPerStoryPointHundredths int              `gorm:"type:integer;not null"`
	Status                       ProjectStatus    `gorm:"type:varchar(16);not null"`
	CreatedAt                    time.Time        `gorm:"type:timestamptz;not null"`
	UpdatedAt                    time.Time        `gorm:"type:timestamptz;not null"`
}
