package service

import (
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
