package domain

import (
	"time"

	"github.com/google/uuid"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// ProjectMember es la participación de un usuario en un proyecto y su existencia es el permiso de
// acceso; se persiste en la tabla project_members (data-model.md sección 2). El ID lo genera la
// aplicación con google/uuid. El índice único compuesto (project_id, user_id) impide que una misma
// persona figure dos veces en el mismo proyecto, y user_id tiene además su propio índice. CreatedAt
// es la fecha de incorporación, por eso no hay columna joined_at, y la propiedad se deriva de
// projects.owner_id, por eso no hay columna is_owner (research.md sección 7). Los campos Project y
// User existen solo para que AutoMigrate cree las claves foráneas de project_id hacia projects.id y
// de user_id hacia users.id: no son columnas y nunca se precargan. CreatedAt y UpdatedAt los
// completa GORM.
type ProjectMember struct {
	ID        uuid.UUID        `gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:idx_project_members_project_user,priority:1"`
	Project   *Project         `gorm:"foreignKey:ProjectID;references:ID"`
	UserID    uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_project_members_project_user,priority:2"`
	User      *authdomain.User `gorm:"foreignKey:UserID;references:ID"`
	CreatedAt time.Time        `gorm:"type:timestamptz;not null"`
	UpdatedAt time.Time        `gorm:"type:timestamptz;not null"`
}
