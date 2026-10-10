package service

import "errors"

// ComparePassword compara con bcrypt la contraseña en claro con el hash guardado: devuelve nil si
// coinciden y domain.ErrInvalidCredentials si no (FR-012, RN-06). Firma mínima para las pruebas de
// T041; la implementación corresponde a T045.
func ComparePassword(hash string, password string) error {
	return errors.New("not implemented")
}
