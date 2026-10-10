// Package domain declara las entidades y las interfaces del módulo de autenticación; no depende de
// ninguna otra capa (Principio III).
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User es la persona que usa el sistema y se persiste en la tabla users (data-model.md sección 1).
// El ID lo genera la aplicación con google/uuid antes de crear el registro, el correo se guarda
// normalizado con un índice único que garantiza que no se repita y la contraseña existe solo como
// hash bcrypt de largo fijo. El modelo no declara colecciones hacia sessions ni auth_events, y
// CreatedAt y UpdatedAt los completa GORM.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	FullName     string    `gorm:"type:varchar(100);not null"`
	Email        string    `gorm:"type:varchar(254);not null;uniqueIndex"`
	PasswordHash string    `gorm:"type:varchar(60);not null"`
	CreatedAt    time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt    time.Time `gorm:"type:timestamptz;not null"`
}
