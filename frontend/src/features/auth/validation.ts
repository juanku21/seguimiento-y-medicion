// Reglas de RC-02 del formulario de registro, coherentes con backend/internal/auth/delivery/
// validation.go. Cada función devuelve el mensaje de error en español o null si el valor es válido.

// Valores límite de la entrada del registro, declarados en un único lugar. MAX_PASSWORD_BYTES es el
// tope que admite bcrypt en el backend.
const MIN_FULL_NAME_LENGTH = 2;
const MAX_FULL_NAME_LENGTH = 100;
const MIN_PASSWORD_LENGTH = 8;
const MAX_PASSWORD_LENGTH = 72;
const MAX_PASSWORD_BYTES = 72;
const MAX_EMAIL_LENGTH = 254;

// Mensajes idénticos a los del backend. Ninguno repite el valor recibido, de modo que la contraseña
// nunca aparece en pantalla (FR-010).
const INVALID_FULL_NAME_MESSAGE = `El nombre debe tener entre ${MIN_FULL_NAME_LENGTH} y ${MAX_FULL_NAME_LENGTH} caracteres.`;
const INVALID_EMAIL_MESSAGE = `El correo debe tener un formato válido y hasta ${MAX_EMAIL_LENGTH} caracteres.`;
const PASSWORD_LENGTH_MESSAGE = `La contraseña debe tener entre ${MIN_PASSWORD_LENGTH} y ${MAX_PASSWORD_LENGTH} caracteres.`;
const PASSWORD_CONTENT_MESSAGE =
  "La contraseña debe incluir al menos una letra y un número.";

// Dirección de correo sin nombre para mostrar: parte local y dominio como secuencias de átomos
// separadas por puntos, según la gramática dot-atom que acepta net/mail.ParseAddress. Los átomos
// admiten los caracteres ASCII permitidos por la RFC 5322 y cualquier carácter no ASCII.
const ATOM = /[A-Za-z0-9!#$%&'*+/=?^_`{|}~\-\u{80}-\u{10FFFF}]+/u.source;
const DOT_ATOM = String.raw`${ATOM}(?:\.${ATOM})*`;
const EMAIL_PATTERN = new RegExp(`^${DOT_ATOM}@${DOT_ATOM}$`, "u");

// Letra y dígito con las mismas categorías Unicode que unicode.IsLetter (L) y unicode.IsDigit (Nd).
const LETTER_PATTERN = /\p{L}/u;
const DIGIT_PATTERN = /\p{Nd}/u;

// Cuenta caracteres por punto de código, como utf8.RuneCountInString en el backend: un emoji vale
// uno aunque ocupe dos unidades UTF-16.
function countCodePoints(value: string): number {
  return [...value].length;
}

// Valida el nombre completo: entre 2 y 100 caracteres tras recortar los espacios extremos; vacío o
// solo de espacios equivale a ausente (RN-03, FR-002).
export function validateFullName(value: string): string | null {
  const length = countCodePoints(value.trim());
  if (length < MIN_FULL_NAME_LENGTH || length > MAX_FULL_NAME_LENGTH) {
    return INVALID_FULL_NAME_MESSAGE;
  }
  return null;
}

// Valida que el correo tenga formato válido, sin nombre para mostrar, y hasta 254 caracteres
// (FR-003).
export function validateEmail(value: string): string | null {
  const tooLong = countCodePoints(value) > MAX_EMAIL_LENGTH;
  if (value === "" || tooLong || !EMAIL_PATTERN.test(value)) {
    return INVALID_EMAIL_MESSAGE;
  }
  return null;
}

// Valida el rango de 8 a 72 caracteres, el tope de 72 bytes en UTF-8 y la composición mínima de
// la contraseña. El exceso de bytes comparte el mensaje del rango; la falta de letra o de número
// tiene un mensaje propio (RN-04, FR-006).
export function validatePassword(value: string): string | null {
  const length = countCodePoints(value);
  const outOfRange =
    length < MIN_PASSWORD_LENGTH || length > MAX_PASSWORD_LENGTH;
  const exceedsBcryptLimit =
    new TextEncoder().encode(value).length > MAX_PASSWORD_BYTES;
  if (outOfRange || exceedsBcryptLimit) {
    return PASSWORD_LENGTH_MESSAGE;
  }

  if (!LETTER_PATTERN.test(value) || !DIGIT_PATTERN.test(value)) {
    return PASSWORD_CONTENT_MESSAGE;
  }
  return null;
}
