// Reglas del formulario de proyecto (RN-04, RN-06, RN-07, FR-003), coherentes con
// backend/internal/projects/delivery/validation.go. Cada función devuelve el mensaje de error en
// español o null si el valor es válido.

// Valores límite de la entrada de proyectos, declarados en un único lugar. Se exportan para que el
// formulario muestre los mismos límites que se validan.
export const MIN_PROJECT_NAME_LENGTH = 1;
export const MAX_PROJECT_NAME_LENGTH = 100;
export const MAX_DESCRIPTION_LENGTH = 1000;
export const MIN_HOURS_PER_STORY_POINT = 0.01;
export const MAX_HOURS_PER_STORY_POINT = 40;
export const MAX_HOURS_PER_STORY_POINT_DECIMALS = 2;

// Mensajes idénticos a los del backend en lo que comparten; las fechas solo exigen presencia porque
// el campo de fecha del navegador ya entrega el formato AAAA-MM-DD.
const INVALID_PROJECT_NAME_MESSAGE = `El nombre debe tener entre ${MIN_PROJECT_NAME_LENGTH} y ${MAX_PROJECT_NAME_LENGTH} caracteres.`;
const INVALID_DESCRIPTION_MESSAGE = `La descripción admite hasta ${MAX_DESCRIPTION_LENGTH} caracteres.`;
const MISSING_START_DATE_MESSAGE = "La fecha de inicio es obligatoria.";
const MISSING_PLANNED_END_DATE_MESSAGE =
  "La fecha de finalización prevista es obligatoria.";
const PLANNED_END_BEFORE_START_MESSAGE =
  "La fecha de finalización prevista no puede ser anterior a la fecha de inicio.";
const INVALID_HOURS_PER_STORY_POINT_MESSAGE = `El factor de horas por Story Point debe ser mayor que 0 y menor o igual a ${MAX_HOURS_PER_STORY_POINT}, con hasta ${MAX_HOURS_PER_STORY_POINT_DECIMALS} decimales.`;

// Factor escrito con dígitos y un punto decimal opcional seguido de hasta los decimales admitidos;
// deja fuera negativos, comas, notación exponencial y decimales de más.
const HOURS_PER_STORY_POINT_PATTERN = new RegExp(
  `^(\\d+)(?:\\.(\\d{1,${MAX_HOURS_PER_STORY_POINT_DECIMALS}}))?$`,
);

// Cuenta caracteres por punto de código, como utf8.RuneCountInString en el backend: un emoji vale
// uno aunque ocupe dos unidades UTF-16.
function countCodePoints(value: string): number {
  return [...value].length;
}

// Valida el nombre del proyecto: entre 1 y 100 caracteres tras recortar los espacios extremos.
export function validateProjectName(value: string): string | null {
  const length = countCodePoints(value.trim());
  if (length < MIN_PROJECT_NAME_LENGTH || length > MAX_PROJECT_NAME_LENGTH) {
    return INVALID_PROJECT_NAME_MESSAGE;
  }
  return null;
}

// Valida la descripción opcional: hasta 1000 caracteres tras recortar los espacios extremos.
export function validateDescription(value: string): string | null {
  if (countCodePoints(value.trim()) > MAX_DESCRIPTION_LENGTH) {
    return INVALID_DESCRIPTION_MESSAGE;
  }
  return null;
}

// Valida que la fecha de inicio (YYYY-MM-DD) esté presente.
export function validateStartDate(value: string): string | null {
  return value === "" ? MISSING_START_DATE_MESSAGE : null;
}

// Valida que la fecha de finalización prevista esté presente y no sea anterior a la de inicio. Las
// fechas AAAA-MM-DD se ordenan igual como texto que como calendario; sin fecha de inicio no hay con
// qué comparar.
export function validatePlannedEndDate(
  startDate: string,
  plannedEndDate: string,
): string | null {
  if (plannedEndDate === "") {
    return MISSING_PLANNED_END_DATE_MESSAGE;
  }
  if (startDate !== "" && plannedEndDate < startDate) {
    return PLANNED_END_BEFORE_START_MESSAGE;
  }
  return null;
}

// Valida el factor tal como se escribió: mayor que 0, hasta 40 y con hasta 2 decimales, sin
// redondear. El rango se compara en centésimas enteras, armadas a partir del texto, para evitar los
// errores de redondeo de los números de punto flotante.
export function validateHoursPerStoryPoint(value: string): string | null {
  const match = HOURS_PER_STORY_POINT_PATTERN.exec(value);
  if (match === null) {
    return INVALID_HOURS_PER_STORY_POINT_MESSAGE;
  }
  const scale = 10 ** MAX_HOURS_PER_STORY_POINT_DECIMALS;
  const fraction = (match[2] ?? "").padEnd(
    MAX_HOURS_PER_STORY_POINT_DECIMALS,
    "0",
  );
  const hundredths = Number(match[1]) * scale + Number(fraction);
  const outOfRange =
    hundredths < Math.round(MIN_HOURS_PER_STORY_POINT * scale) ||
    hundredths > MAX_HOURS_PER_STORY_POINT * scale;
  return outOfRange ? INVALID_HOURS_PER_STORY_POINT_MESSAGE : null;
}
