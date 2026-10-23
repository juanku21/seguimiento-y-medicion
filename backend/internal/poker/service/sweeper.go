package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Sweep ejecuta una pasada del barrido periódico (research.md sección 13). Por ahora cubre las
// gracias vencidas: cada persona cuya última conexión cayó hace la gracia o más pasa a desconectada
// y se publica la sesión. Una sesión que ya está cerrada no necesita la desconexión y se saltea; los
// demás errores no cortan la pasada y se devuelven juntos al final.
func (s *SessionService) Sweep(ctx context.Context) error {
	now := s.now()
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
	return errors.Join(errs...)
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
