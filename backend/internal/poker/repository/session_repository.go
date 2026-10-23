// Package repository implementa con GORM la persistencia del módulo de Planning Poker.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// errNotImplemented es el error de las firmas mínimas de T014; T022 y las tareas de cada historia
// reemplazan cada cuerpo por su implementación.
var errNotImplemented = errors.New("not implemented")

// SessionRepository implementa domain.SessionRepository con GORM sobre estimation_sessions,
// estimation_rounds, estimation_participants y estimation_votes. Firma mínima de T014: la
// implementa T022.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository crea el repositorio de sesiones de estimación sobre la conexión recibida.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// FindAccessible resuelve en una consulta la sesión y la membresía vigente del titular. Firma mínima
// de T014: la implementa T022.
func (r *SessionRepository) FindAccessible(ctx context.Context, sessionID, userID uuid.UUID) (*domain.SessionAccess, error) {
	return nil, errNotImplemented
}

// Open crea la sesión con la ronda 1 y a quien abre como facilitador. Firma mínima de T014: la
// implementa T022.
func (r *SessionRepository) Open(ctx context.Context, storyID, openerID uuid.UUID, now time.Time) (*domain.EstimationSession, error) {
	return nil, errNotImplemented
}

// Connect da de alta o reconecta al participante. Firma mínima de T014: la implementa T022.
func (r *SessionRepository) Connect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// Disconnect marca al participante desconectado. Firma mínima de T014: la implementa T022.
func (r *SessionRepository) Disconnect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// Vote registra o cambia la carta de un participante. Firma mínima de T014.
func (r *SessionRepository) Vote(ctx context.Context, sessionID, userID uuid.UUID, card domain.Card, now time.Time) error {
	return errNotImplemented
}

// Reveal revela manualmente la ronda vigente. Firma mínima de T014.
func (r *SessionRepository) Reveal(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// StartRound abre la ronda siguiente. Firma mínima de T014.
func (r *SessionRepository) StartRound(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// Finalize cierra la sesión con el valor acordado. Firma mínima de T014.
func (r *SessionRepository) Finalize(ctx context.Context, sessionID, userID uuid.UUID, storyPoints int, now time.Time) error {
	return errNotImplemented
}

// Cancel cancela la sesión a pedido del facilitador. Firma mínima de T014.
func (r *SessionRepository) Cancel(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// TakeFacilitator transfiere el rol de facilitador. Firma mínima de T014.
func (r *SessionRepository) TakeFacilitator(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// AutoCancel cancela la sesión si corresponde una cancelación del sistema. Firma mínima de T014.
func (r *SessionRepository) AutoCancel(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	return false, errNotImplemented
}

// ResetPresence marca desconectados a los participantes de las sesiones abiertas. Firma mínima de
// T014: la implementa T022.
func (r *SessionRepository) ResetPresence(ctx context.Context, now time.Time) error {
	return errNotImplemented
}

// ListOpenForSweep devuelve las sesiones abiertas para el barrido. Firma mínima de T014.
func (r *SessionRepository) ListOpenForSweep(ctx context.Context) ([]domain.OpenSessionStatus, error) {
	return nil, errNotImplemented
}

// ListRevokedParticipants devuelve los participantes conectados que ya no son integrantes. Firma
// mínima de T014.
func (r *SessionRepository) ListRevokedParticipants(ctx context.Context) ([]domain.ParticipantRef, error) {
	return nil, errNotImplemented
}

// FindSnapshot devuelve la instantánea completa de la sesión. Firma mínima de T014: la implementa
// T022.
func (r *SessionRepository) FindSnapshot(ctx context.Context, sessionID uuid.UUID) (*domain.SessionSnapshot, error) {
	return nil, errNotImplemented
}

// ListForStory devuelve las sesiones cerradas de la historia. Firma mínima de T014.
func (r *SessionRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.SessionSummary, error) {
	return nil, errNotImplemented
}

// FindOpenForStory devuelve la sesión abierta de la historia. Firma mínima de T014.
func (r *SessionRepository) FindOpenForStory(ctx context.Context, storyID uuid.UUID) (*domain.OpenSessionSummary, error) {
	return nil, errNotImplemented
}

// Verificación en compilación de que el repositorio cumple el contrato del dominio.
var _ domain.SessionRepository = (*SessionRepository)(nil)
