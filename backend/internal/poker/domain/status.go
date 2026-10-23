package domain

import "errors"

// SessionStatus es el estado de una sesión de estimación; se guarda como texto en la columna
// estimation_sessions.status y no como enum de PostgreSQL (data-model.md sección 5). Los valores
// están en inglés y la etiqueta en español vive en el frontend.
type SessionStatus string

// Estados posibles de una sesión: open (Abierta), finished (Finalizada) y canceled (Cancelada). Toda
// sesión nace open; finished y canceled son terminales.
const (
	SessionStatusOpen     SessionStatus = "open"
	SessionStatusFinished SessionStatus = "finished"
	SessionStatusCanceled SessionStatus = "canceled"
)

// EnsureOpen admite la acción si la sesión está open; si está cerrada devuelve SessionClosedError con
// el estado y con el motivo de cancelación y el valor acordado recibidos (RN-22, data-model.md
// sección 6). Firma mínima de T010: la implementa T018.
func (s SessionStatus) EnsureOpen(reason *CancelReason, agreedStoryPoints *int) error {
	return errors.New("not implemented")
}

// RoundStatus es el estado de una ronda de votación; se guarda como texto y no como enum de
// PostgreSQL (data-model.md sección 5).
type RoundStatus string

// Estados posibles de una ronda: voting (En votación) y revealed (Revelada). Toda ronda nace voting.
const (
	RoundStatusVoting   RoundStatus = "voting"
	RoundStatusRevealed RoundStatus = "revealed"
)

// CancelReason es el motivo de una cancelación hecha por el sistema; se guarda como texto en la
// columna estimation_sessions.cancel_reason y no como enum de PostgreSQL (data-model.md sección 5).
type CancelReason string

// Motivos posibles de cancelación por el sistema: inactivity (Por inactividad), story_completed
// (Historia completada) y project_finished (Proyecto finalizado).
const (
	CancelReasonInactivity      CancelReason = "inactivity"
	CancelReasonStoryCompleted  CancelReason = "story_completed"
	CancelReasonProjectFinished CancelReason = "project_finished"
)

// OutcomeKind es la clase de resultado de una ronda revelada (data-model.md sección 5); no se
// persiste como enum de PostgreSQL.
type OutcomeKind string

// Clases de resultado posibles: consensus (Consenso), divergence (Divergencia) y no_numeric_votes
// (Sin votos numéricos).
const (
	OutcomeKindConsensus      OutcomeKind = "consensus"
	OutcomeKindDivergence     OutcomeKind = "divergence"
	OutcomeKindNoNumericVotes OutcomeKind = "no_numeric_votes"
)
