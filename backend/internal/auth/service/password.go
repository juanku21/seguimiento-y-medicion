package service

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
)

// HashPassword devuelve el hash bcrypt de la contraseña con el coste por defecto (FR-010, RN-05).
// Si bcrypt la rechaza, el error se envuelve sin incluir la contraseña y no se devuelve hash.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// ComparePassword compara con bcrypt la contraseña en claro con el hash guardado: devuelve nil si
// coinciden y domain.ErrInvalidCredentials si no (FR-012, RN-06). Cualquier otro fallo de bcrypt,
// como un hash guardado con formato inválido, se envuelve sin incluir la contraseña.
func ComparePassword(hash string, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return domain.ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("comparing password: %w", err)
	}
	return nil
}
