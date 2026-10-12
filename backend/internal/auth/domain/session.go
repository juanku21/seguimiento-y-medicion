package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionLifetime es el plazo fijo de una sesión desde su emisión; no se renueva con el uso
// (FR-013, FR-018).
const SessionLifetime = 8 * time.Hour

// Session es el permiso temporal de una persona para operar y se persiste en la tabla sessions
// (data-model.md sección 2). El ID es el claim jti del JWT y lo genera la aplicación; CreatedAt es
// el momento de emisión y ExpiresAt queda fijo en CreatedAt más 8 horas. RevokedAt es nulo mientras
// el titular no la cierre. No hay columna de estado: vigente, cerrada y vencida se derivan de
// RevokedAt y ExpiresAt. El campo User existe solo para que AutoMigrate cree la clave foránea de
// user_id hacia users.id (relación de pertenencia); no es una columna, no se precarga y User no
// declara la colección inversa.
type Session struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index"`
	User      *User      `gorm:"foreignKey:UserID;references:ID"`
	ExpiresAt time.Time  `gorm:"type:timestamptz;not null"`
	RevokedAt *time.Time `gorm:"type:timestamptz"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt time.Time  `gorm:"type:timestamptz;not null"`
}

// IsActive indica si la sesión está vigente en el instante now: lo está únicamente si no fue
// revocada (RevokedAt nulo) y su vencimiento todavía no llegó (ExpiresAt posterior a now), de forma
// estricta (RN-07, data-model.md sección 2). VENCIDA y CERRADA son estados terminales: ninguno de
// los dos vuelve a ser vigente con el paso del tiempo.
func (s Session) IsActive(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}
