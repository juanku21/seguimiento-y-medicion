import type { StoryPriority } from "@/types/story";
import { STORY_PRIORITY_LABELS } from "@/utils/story-labels";

// Reglas del formulario de historia (FR-002 a FR-004, FR-007), coherentes con
// backend/internal/backlog/delivery/validation.go. Cada función devuelve el mensaje de error en
// español o null si el valor es válido.

// Valores límite de la entrada de historias, declarados en un único lugar. Se exportan para que el
// formulario muestre los mismos límites que se validan.
export const MAX_STORY_TITLE_LENGTH = 150;
export const MAX_STORY_DESCRIPTION_LENGTH = 2000;
export const MAX_ACCEPTANCE_CRITERION_LENGTH = 500;

// Mensajes idénticos a los del backend (data-model.md, sección 2). El de prioridad agrega el nombre
// del campo, porque en el formulario el mensaje se muestra sin la clave que lo acompaña en la API.
const TITLE_REQUIRED_MESSAGE = "El título es obligatorio.";
const TITLE_TOO_LONG_MESSAGE = `El título admite como máximo ${MAX_STORY_TITLE_LENGTH} caracteres.`;
const DESCRIPTION_TOO_LONG_MESSAGE = `La descripción admite como máximo ${MAX_STORY_DESCRIPTION_LENGTH} caracteres.`;
const INVALID_PRIORITY_MESSAGE =
  "La prioridad es obligatoria. Los valores admitidos son Alta, Media y Baja.";
const CRITERION_REQUIRED_MESSAGE =
  "Cada criterio de aceptación debe tener contenido.";
const CRITERION_TOO_LONG_MESSAGE = `Cada criterio de aceptación admite como máximo ${MAX_ACCEPTANCE_CRITERION_LENGTH} caracteres.`;

// Cuenta caracteres por punto de código, como utf8.RuneCountInString en el backend: un emoji vale
// uno aunque ocupe dos unidades UTF-16.
function countCodePoints(value: string): number {
  return [...value].length;
}

// Indica si el texto es uno de los valores de prioridad de la API, tomados de las etiquetas.
function isStoryPriority(value: string): value is StoryPriority {
  return Object.hasOwn(STORY_PRIORITY_LABELS, value);
}

// Valida el título: entre 1 y 150 caracteres tras recortar los espacios extremos.
export function validateStoryTitle(value: string): string | null {
  const length = countCodePoints(value.trim());
  if (length === 0) {
    return TITLE_REQUIRED_MESSAGE;
  }
  if (length > MAX_STORY_TITLE_LENGTH) {
    return TITLE_TOO_LONG_MESSAGE;
  }
  return null;
}

// Valida la descripción opcional: hasta 2000 caracteres tras recortar los espacios extremos.
export function validateStoryDescription(value: string): string | null {
  if (countCodePoints(value.trim()) > MAX_STORY_DESCRIPTION_LENGTH) {
    return DESCRIPTION_TOO_LONG_MESSAGE;
  }
  return null;
}

// Valida que la prioridad sea uno de los valores admitidos de la API: high, medium o low.
export function validateStoryPriority(value: string): string | null {
  return isStoryPriority(value) ? null : INVALID_PRIORITY_MESSAGE;
}

// Valida un criterio de aceptación: entre 1 y 500 caracteres tras recortar los espacios extremos.
export function validateAcceptanceCriterion(value: string): string | null {
  const length = countCodePoints(value.trim());
  if (length === 0) {
    return CRITERION_REQUIRED_MESSAGE;
  }
  if (length > MAX_ACCEPTANCE_CRITERION_LENGTH) {
    return CRITERION_TOO_LONG_MESSAGE;
  }
  return null;
}
