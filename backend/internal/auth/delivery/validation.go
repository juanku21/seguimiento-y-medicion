package delivery

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Valores límite de la entrada del registro, declarados en un único lugar (research.md, sección 13).
// maxPasswordBytes es el tope que admite bcrypt: superarlo se rechaza como entrada inválida antes de
// llegar al hash, nunca como error de bcrypt.
const (
	minFullNameLength = 2
	maxFullNameLength = 100
	minPasswordLength = 8
	maxPasswordLength = 72
	maxPasswordBytes  = 72
	maxEmailLength    = 254
)

// Errores de validación con mensajes en español dirigidos a la persona. Ninguno repite el valor
// recibido, de modo que la contraseña nunca aparece en una respuesta (FR-010). Los límites se toman
// de las constantes para no repetirlos en el texto.
var (
	errInvalidFullName = fmt.Errorf("El nombre debe tener entre %d y %d caracteres.",
		minFullNameLength, maxFullNameLength)
	errInvalidEmail = fmt.Errorf("El correo debe tener un formato válido y hasta %d caracteres.",
		maxEmailLength)
	errPasswordLength = fmt.Errorf("La contraseña debe tener entre %d y %d caracteres.",
		minPasswordLength, maxPasswordLength)
	errPasswordContent = errors.New("La contraseña debe incluir al menos una letra y un número.")
)

// NormalizeEmail recorta los espacios extremos del correo y lo pasa a minúsculas antes de que
// llegue a la capa de servicio (FR-004, RN-02). No altera puntos ni alias.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateFullName exige un nombre de entre minFullNameLength y maxFullNameLength runas, contadas
// tras recortar los espacios extremos; vacío o solo de espacios equivale a ausente (FR-002, RN-03).
func ValidateFullName(fullName string) error {
	length := utf8.RuneCountInString(strings.TrimSpace(fullName))
	if length < minFullNameLength || length > maxFullNameLength {
		return errInvalidFullName
	}
	return nil
}

// ValidateEmail exige un correo con formato válido según net/mail.ParseAddress, sin nombre para
// mostrar y de hasta maxEmailLength caracteres (FR-003). Exigir que la dirección analizada coincida con la
// entrada descarta los formatos con ángulos, comentarios o varias direcciones.
func ValidateEmail(email string) error {
	if email == "" || utf8.RuneCountInString(email) > maxEmailLength {
		return errInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" || addr.Address != email {
		return errInvalidEmail
	}
	return nil
}

// ValidatePassword exige una contraseña de entre minPasswordLength y maxPasswordLength runas, de
// como máximo maxPasswordBytes bytes en UTF-8 y con al menos una letra y un dígito (FR-006, RN-04).
// El exceso de bytes comparte el mensaje del rango; la falta de composición tiene un mensaje propio.
func ValidatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	outOfRange := length < minPasswordLength || length > maxPasswordLength
	exceedsBcryptLimit := len(password) > maxPasswordBytes
	if outOfRange || exceedsBcryptLimit {
		return errPasswordLength
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		hasLetter = hasLetter || unicode.IsLetter(r)
		hasDigit = hasDigit || unicode.IsDigit(r)
	}
	if !hasLetter || !hasDigit {
		return errPasswordContent
	}
	return nil
}
