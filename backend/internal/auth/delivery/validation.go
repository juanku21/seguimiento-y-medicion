package delivery

import "errors"

// NormalizeEmail recorta los espacios extremos del correo y lo pasa a minúsculas antes de que
// llegue a la capa de servicio (FR-004, RN-02). Firma mínima: la implementa T029.
func NormalizeEmail(email string) string {
	return ""
}

// ValidateFullName exige un nombre de entre 2 y 100 runas contadas tras recortar los espacios
// extremos; vacío o solo de espacios equivale a ausente (FR-002, RN-03). Firma mínima: la implementa
// T029.
func ValidateFullName(fullName string) error {
	return errors.New("not implemented")
}

// ValidateEmail exige un correo con formato válido según net/mail.ParseAddress, sin nombre para
// mostrar y de hasta 254 caracteres (FR-003). Firma mínima: la implementa T029.
func ValidateEmail(email string) error {
	return errors.New("not implemented")
}

// ValidatePassword exige una contraseña de entre 8 y 72 runas, de como máximo 72 bytes en UTF-8 y
// con al menos una letra y un dígito (FR-006, RN-04). Firma mínima: la implementa T029.
func ValidatePassword(password string) error {
	return errors.New("not implemented")
}
