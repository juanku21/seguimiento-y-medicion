// Reglas del formulario de sprint (FR-002 a FR-004), coherentes con
// backend/internal/sprints/delivery/validation.go. Cada función devuelve el mensaje de error en
// español o null si el valor es válido.

// Longitudes máximas del nombre y del Sprint Goal, declaradas en un único lugar (RN-03). Se exportan
// para que el formulario muestre los mismos límites que se validan.
export const MAX_SPRINT_NAME_LENGTH = 100;
export const MAX_SPRINT_GOAL_LENGTH = 500;

// Mensajes idénticos a los del backend (data-model.md, reglas de validación); los límites se toman
// de las constantes para no repetirlos en el texto.
const NAME_REQUIRED_MESSAGE = "El nombre es obligatorio.";
const NAME_TOO_LONG_MESSAGE = `El nombre admite como máximo ${MAX_SPRINT_NAME_LENGTH} caracteres.`;
const GOAL_REQUIRED_MESSAGE = "El Sprint Goal es obligatorio.";
const GOAL_TOO_LONG_MESSAGE = `El Sprint Goal admite como máximo ${MAX_SPRINT_GOAL_LENGTH} caracteres.`;
const START_DATE_REQUIRED_MESSAGE = "La fecha de inicio es obligatoria.";
const START_DATE_INVALID_MESSAGE =
  "La fecha de inicio debe ser una fecha válida con el formato AAAA-MM-DD.";
const END_DATE_REQUIRED_MESSAGE = "La fecha de fin es obligatoria.";
const END_DATE_INVALID_MESSAGE =
  "La fecha de fin debe ser una fecha válida con el formato AAAA-MM-DD.";
const END_DATE_NOT_AFTER_START_MESSAGE =
  "La fecha de fin debe ser posterior a la de inicio.";

// Formato AAAA-MM-DD, el valor de un campo de tipo fecha y el layout que acepta el backend.
const DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

// Cuenta caracteres por punto de código, como utf8.RuneCountInString en el backend: un emoji vale
// uno aunque ocupe dos unidades UTF-16.
function countCodePoints(value: string): number {
  return [...value].length;
}

// Valida un texto obligatorio recortado y con longitud máxima, devolviendo el mensaje que
// corresponda o null si es válido.
function validateText(
  value: string,
  maxLength: number,
  requiredMessage: string,
  tooLongMessage: string,
): string | null {
  const length = countCodePoints(value.trim());
  if (length === 0) {
    return requiredMessage;
  }
  if (length > maxLength) {
    return tooLongMessage;
  }
  return null;
}

// Indica si el texto es una fecha de calendario existente con el formato AAAA-MM-DD. Una fecha
// inexistente, como 2026-02-30, se descarta comprobando que el día armado no se corra de mes.
function isCalendarDate(value: string): boolean {
  const match = DATE_PATTERN.exec(value);
  if (match === null) {
    return false;
  }
  const [year, month, day] = match.slice(1).map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
}

// Valida una fecha obligatoria con el formato AAAA-MM-DD; el texto no se recorta, como en el
// backend.
function validateDate(
  value: string,
  requiredMessage: string,
  invalidMessage: string,
): string | null {
  if (value === "") {
    return requiredMessage;
  }
  return isCalendarDate(value) ? null : invalidMessage;
}

// Valida el nombre del sprint: entre 1 y 100 caracteres tras recortar los espacios extremos.
export function validateSprintName(value: string): string | null {
  return validateText(
    value,
    MAX_SPRINT_NAME_LENGTH,
    NAME_REQUIRED_MESSAGE,
    NAME_TOO_LONG_MESSAGE,
  );
}

// Valida el Sprint Goal: entre 1 y 500 caracteres tras recortar los espacios extremos.
export function validateSprintGoal(value: string): string | null {
  return validateText(
    value,
    MAX_SPRINT_GOAL_LENGTH,
    GOAL_REQUIRED_MESSAGE,
    GOAL_TOO_LONG_MESSAGE,
  );
}

// Valida la fecha de inicio: obligatoria y con formato AAAA-MM-DD.
export function validateSprintStartDate(value: string): string | null {
  return validateDate(
    value,
    START_DATE_REQUIRED_MESSAGE,
    START_DATE_INVALID_MESSAGE,
  );
}

// Valida la fecha de fin: obligatoria, con formato AAAA-MM-DD y estrictamente posterior a la de
// inicio. El orden solo se verifica si la de inicio también es válida, como en el backend; con el
// formato AAAA-MM-DD la comparación de textos equivale a la de fechas.
export function validateSprintEndDate(
  startDate: string,
  endDate: string,
): string | null {
  const message = validateDate(
    endDate,
    END_DATE_REQUIRED_MESSAGE,
    END_DATE_INVALID_MESSAGE,
  );
  if (message !== null) {
    return message;
  }
  if (isCalendarDate(startDate) && endDate <= startDate) {
    return END_DATE_NOT_AFTER_START_MESSAGE;
  }
  return null;
}
