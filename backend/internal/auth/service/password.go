package service

import "errors"

// HashPassword devuelve el hash bcrypt de la contraseña (FR-010, RN-05). Firma mínima creada por
// T026 para que compilen las pruebas; la implementación corresponde a T030.
func HashPassword(password string) (string, error) {
	return "", errors.New("not implemented")
}
