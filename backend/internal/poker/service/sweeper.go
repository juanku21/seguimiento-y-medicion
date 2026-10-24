package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Sweep ejecuta una pasada del barrido periódico (research.md sección 13): las gracias vencidas, la
// membresía revocada y los cambios de la historia, en ese orden. Una sesión que ya está cerrada no
// necesita la desconexión y se saltea; los demás errores no cortan la pasada y se devuelven juntos al
// final.
func (s *SessionService) Sweep(ctx context.Context) error {
	now := s.now()
	var errs []error
	errs = append(errs, s.sweepExpired(ctx, now)...)
	errs = append(errs, s.sweepRevoked(ctx, now)...)
	errs = append(errs, s.sweepStoryChanges(ctx)...)
	return errors.Join(errs...)
}

// sweepExpired desconecta a cada persona cuya última conexión cayó hace la gracia o más y publica la
// sesión (FR-014).
func (s *SessionService) sweepExpired(ctx context.Context, now time.Time) []error {
	var errs []error
	for _, key := range s.presence.Expired(now) {
		err := s.repo.Disconnect(ctx, key.SessionID, key.UserID, now)
		var closed *domain.SessionClosedError
		if errors.As(err, &closed) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("disconnecting expired participant: %w", err))
			continue
		}
		if err := s.publish(ctx, key.SessionID); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// sweepRevoked desconecta a los participantes conectados que dejaron de ser integrantes del proyecto
// (FR-019): registra la desconexión, que evalúa la revelación automática y conserva sus votos, la
// quita de la presencia para que la gracia no la repita, anota la revocación y cierra sus canales,
// que envían removed al terminar, y recién entonces publica la sesión a los demás.
func (s *SessionService) sweepRevoked(ctx context.Context, now time.Time) []error {
	refs, err := s.repo.ListRevokedParticipants(ctx)
	if err != nil {
		return []error{err}
	}

	var errs []error
	for _, ref := range refs {
		key := PresenceKey{SessionID: ref.SessionID, UserID: ref.UserID}
		err := s.repo.Disconnect(ctx, key.SessionID, key.UserID, now)
		var closed *domain.SessionClosedError
		if errors.As(err, &closed) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("disconnecting revoked participant: %w", err))
			continue
		}
		s.presence.Forget(key.SessionID, key.UserID)
		s.setRevoked(key, true)
		s.hub.CloseUser(key.SessionID, key.UserID)
		if err := s.publish(ctx, key.SessionID); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// sweepStoryChanges publica las sesiones abiertas cuya historia cambió desde la pasada anterior,
// comparando user_stories.updated_at con el último visto (FR-074). Una sesión que aparece por primera
// vez también se publica, porque no hay con qué compararla; publicar no es un hecho, así que no sube
// la versión ni reinicia la inactividad. Las sesiones que dejaron de estar abiertas se olvidan.
func (s *SessionService) sweepStoryChanges(ctx context.Context) []error {
	sessions, err := s.repo.ListOpenForSweep(ctx)
	if err != nil {
		return []error{err}
	}

	s.storySeenMu.Lock()
	changed := []uuid.UUID{}
	seen := make(map[uuid.UUID]time.Time, len(sessions))
	for _, open := range sessions {
		last, ok := s.storySeen[open.SessionID]
		if !ok || !last.Equal(open.StoryUpdatedAt) {
			changed = append(changed, open.SessionID)
		}
		seen[open.SessionID] = open.StoryUpdatedAt
	}
	s.storySeen = seen
	s.storySeenMu.Unlock()

	var errs []error
	for _, sessionID := range changed {
		if err := s.publish(ctx, sessionID); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Run ejecuta Sweep cada intervalo hasta que termina el contexto. Un error de una pasada se
// registra con slog y el barrido sigue con la próxima, sin detener el servidor (Principio VII).
func (s *SessionService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Sweep(ctx); err != nil {
				slog.Error("estimation session sweep failed", "error", err)
			}
		}
	}
}
