package domain

import (
	"time"

	"github.com/google/uuid"
)

// EventType identifica qué ocurrió con una cuenta. Se modela como constantes de Go tipadas y no
// como enum de PostgreSQL, para que un tipo nuevo no exija una migración (data-model.md sección 3).
type EventType string

// Tipos de evento que registra la feature de autenticación (FR-029).
const (
	EventAccountCreated EventType = "account_created"
	EventLoginSucceeded EventType = "login_succeeded"
	EventLoginFailed    EventType = "login_failed"
	EventLogout         EventType = "logout"
)

// AuthEvent es algo que ocurrió con las cuentas y que conviene poder revisar después; se persiste
// en la tabla auth_events (data-model.md sección 3). El ID lo genera la aplicación y CreatedAt es
// el momento del evento. UserID es nulo en un inicio de sesión fallido con un correo no registrado,
// caso en que el intento queda rastreable por AttemptedEmail, que solo se completa en login_failed.
// Ninguna columna admite contraseñas (FR-030). El campo User existe solo para que AutoMigrate cree
// la clave foránea de user_id hacia users.id; no es una columna, no se precarga y User no declara
// la colección inversa.
type AuthEvent struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	EventType      EventType  `gorm:"type:varchar(32);not null"`
	UserID         *uuid.UUID `gorm:"type:uuid;index"`
	User           *User      `gorm:"foreignKey:UserID;references:ID"`
	AttemptedEmail *string    `gorm:"type:varchar(254)"`
	CreatedAt      time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt      time.Time  `gorm:"type:timestamptz;not null"`
}
