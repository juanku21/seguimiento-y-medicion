package service

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
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
// coinciden y domain.ErrInvalidCredentials si no (FR-012, RN-06). Firma mínima para las pruebas de
// T041; la implementación corresponde a T045.
func ComparePassword(hash string, password string) error {
	return errors.New("not implemented")
}
