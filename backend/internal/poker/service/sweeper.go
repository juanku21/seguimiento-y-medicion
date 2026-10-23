package service

import (
	"context"
	"errors"
)

// Sweep ejecuta una pasada del barrido periódico: gracias vencidas, cancelaciones del sistema,
// membresía revocada y cambios de la historia (research.md sección 13). Firma mínima de T015: la
// implementa T023.
func (s *SessionService) Sweep(ctx context.Context) error {
	return errors.New("not implemented")
}
