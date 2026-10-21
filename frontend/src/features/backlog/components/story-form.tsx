"use client";

import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import { Input } from "@/components/input";
import { Textarea } from "@/components/textarea";
import { createStory, updateStory } from "@/features/backlog/api";
import {
  AcceptanceCriteriaEditor,
  type CriterionItem,
} from "@/features/backlog/components/acceptance-criteria-editor";
import {
  MAX_STORY_DESCRIPTION_LENGTH,
  MAX_STORY_TITLE_LENGTH,
  isStoryPriority,
  validateAcceptanceCriterion,
  validateStoryDescription,
  validateStoryPriority,
  validateStoryTitle,
} from "@/features/backlog/validation";
import type { FieldError } from "@/types/auth";
import type { StoryDetail, StoryPatch } from "@/types/story";
import {
  STORY_POINTS_SCALE,
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
  UNESTIMATED_LABEL,
  readinessLabel,
  toStoryPoints,
} from "@/utils/story-labels";

// Valores de los campos simples tal como se eligen: prioridad y Story Points son el valor del
// selector ("" significa sin elegir y sin estimar, respectivamente).
type FormValues = {
  title: string;
  description: string;
  priority: string;
  storyPoints: string;
};

type FieldName = keyof FormValues;
type FieldErrors = Partial<Record<FieldName, string>>;
type CriterionErrors = Partial<Record<string, string>>;

const EMPTY_VALUES: FormValues = {
  title: "",
  description: "",
  priority: "",
  storyPoints: "",
};

// Indicaciones de los límites, visibles desde el inicio y armadas con las mismas constantes que
// usa la validación (FR-002, FR-003).
const TITLE_HINT = `Hasta ${MAX_STORY_TITLE_LENGTH} caracteres.`;
const DESCRIPTION_HINT = `Opcional, hasta ${MAX_STORY_DESCRIPTION_LENGTH} caracteres.`;

// Clave con la que el backend informa el error de un criterio, por ejemplo acceptanceCriteria[1].
const CRITERION_FIELD_PATTERN = /^acceptanceCriteria\[(\d+)\]$/;

// Indica si un nombre de campo informado por el backend corresponde a un campo simple del
// formulario.
function isFieldName(value: string): value is FieldName {
  return Object.hasOwn(EMPTY_VALUES, value);
}

// Aplica las reglas del formulario a los campos simples y devuelve solo los que tienen error.
function validateFields(values: FormValues): FieldErrors {
  const errors: FieldErrors = {};
  const checks: [FieldName, string | null][] = [
    ["title", validateStoryTitle(values.title)],
    ["description", validateStoryDescription(values.description)],
    ["priority", validateStoryPriority(values.priority)],
  ];
  for (const [field, message] of checks) {
    if (message) {
      errors[field] = message;
    }
  }
  return errors;
}

// Valida cada criterio y devuelve los errores por id de criterio.
function validateCriteria(items: CriterionItem[]): CriterionErrors {
  const errors: CriterionErrors = {};
  for (const item of items) {
    const message = validateAcceptanceCriterion(item.text);
    if (message) {
      errors[item.id] = message;
    }
  }
  return errors;
}

// Ubica junto a cada campo el detalle que informa el backend: los campos simples por su nombre y
// los criterios por su posición en la lista enviada. Se descartan los que no son del formulario.
function toErrors(fields: FieldError[], items: CriterionItem[]) {
  const fieldErrors: FieldErrors = {};
  const criterionErrors: CriterionErrors = {};
  for (const { field, message } of fields) {
    const criterionMatch = CRITERION_FIELD_PATTERN.exec(field);
    const item = criterionMatch ? items[Number(criterionMatch[1])] : undefined;
    if (item) {
      criterionErrors[item.id] = message;
    } else if (isFieldName(field)) {
      fieldErrors[field] = message;
    }
  }
  return { fieldErrors, criterionErrors };
}

// Convierte la descripción escrita en la que se envía: vacía o solo espacios viaja como null.
function toDescription(value: string): string | null {
  return value.trim() === "" ? null : value;
}

// Valores iniciales de los campos simples: vacíos al registrar o los vigentes de la historia al
// refinarla.
function initialValues(story: StoryDetail | null): FormValues {
  if (!story) {
    return EMPTY_VALUES;
  }
  return {
    title: story.title,
    description: story.description ?? "",
    priority: story.priority,
    storyPoints: "",
  };
}

// Criterios iniciales en edición, con un id estable por posición que no choca con los que genera
// el editor al agregar.
function initialCriteria(story: StoryDetail | null): CriterionItem[] {
  return (story?.acceptanceCriteria ?? []).map((text, index) => ({
    id: `initial-${index}`,
    text,
  }));
}

// Arma el cuerpo de la modificación con solo los campos que difieren de la historia cargada, para
// no pisar los cambios de otro integrante (FR-025). La lista de criterios, si cambió en contenido u
// orden, viaja completa (FR-022).
function toStoryPatch(
  story: StoryDetail,
  values: FormValues,
  criteria: CriterionItem[],
): StoryPatch {
  const patch: StoryPatch = {};
  if (values.title !== story.title) {
    patch.title = values.title;
  }
  if (values.description !== (story.description ?? "")) {
    patch.description = toDescription(values.description);
  }
  if (values.priority !== story.priority && isStoryPriority(values.priority)) {
    patch.priority = values.priority;
  }
  const texts = criteria.map((item) => item.text);
  const sameCriteria =
    texts.length === story.acceptanceCriteria.length &&
    texts.every((text, index) => text === story.acceptanceCriteria[index]);
  if (!sameCriteria) {
    patch.acceptanceCriteria = texts;
  }
  return patch;
}

// Propiedades del formulario de historia: el proyecto en cuyo backlog se registra una historia
// nueva o la historia vigente que se refina.
export type StoryFormProps = { projectId: string } | { story: StoryDetail };

// Formulario de una historia del backlog. Al registrar (003/US1) envía todos los campos, incluidos
// los Story Points; al refinar (003/US5) arranca precargado, no ofrece Story Points ni estado y
// envía solo los campos cambiados. Valida antes de enviar y confirma el título, el estado, los Story
// Points y si quedó lista para planificar. Ante un 401 el cliente HTTP descarta la sesión.
export function StoryForm(props: StoryFormProps) {
  const story = "story" in props ? props.story : null;
  const [values, setValues] = useState<FormValues>(() => initialValues(story));
  const [criteria, setCriteria] = useState<CriterionItem[]>(() =>
    initialCriteria(story),
  );
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [criterionErrors, setCriterionErrors] = useState<CriterionErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [createdStory, setCreatedStory] = useState<StoryDetail | null>(null);
  const submittingRef = useRef(false);

  // Actualiza un campo simple del formulario a partir de lo que se escribió o eligió.
  function updateField(field: FieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Envía la creación o la modificación. La referencia descarta un segundo envío mientras el primero sigue en curso,
  // aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors = validateFields(values);
    const itemErrors = validateCriteria(criteria);
    setFieldErrors(errors);
    setCriterionErrors(itemErrors);
    setGeneralError(null);
    const hasErrors =
      Object.keys(errors).length > 0 || Object.keys(itemErrors).length > 0;
    if (hasErrors || !isStoryPriority(values.priority)) {
      return;
    }

    // Al registrar se envía lo escrito; la descripción vacía viaja como null y los Story Points sin
    // estimar también, para distinguirlos del 0 (FR-015). Al refinar se envían solo los cambios.
    submittingRef.current = true;
    setIsSubmitting(true);
    const result =
      "story" in props
        ? await updateStory(
            props.story.id,
            toStoryPatch(props.story, values, criteria),
          )
        : await createStory(props.projectId, {
            title: values.title,
            description: toDescription(values.description),
            priority: values.priority,
            storyPoints: toStoryPoints(values.storyPoints),
            acceptanceCriteria: criteria.map((item) => item.text),
          });
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setCreatedStory(result.data);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo o criterio.
    const backendErrors = toErrors(result.error.fields, criteria);
    setFieldErrors(backendErrors.fieldErrors);
    setCriterionErrors(backendErrors.criterionErrors);
    setGeneralError(result.error.message);
  }

  // Confirmación de la creación o de la modificación: se muestran los datos devueltos por el backend en el lugar del
  // formulario.
  if (createdStory) {
    return (
      <Alert variant="success">
        <p className="font-medium">
          {story ? "Historia actualizada" : "Historia creada"}
        </p>
        <p>{createdStory.title}</p>
        <p>
          Estado: {STORY_STATUS_LABELS[createdStory.status]} · Story Points:{" "}
          {createdStory.storyPoints ?? UNESTIMATED_LABEL}
        </p>
        <p>{readinessLabel(createdStory.readiness.ready)}</p>
      </Alert>
    );
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {generalError && <Alert>{generalError}</Alert>}
      <Input
        label="Título"
        name="title"
        autoComplete="off"
        value={values.title}
        onChange={(event) => updateField("title", event.target.value)}
        hint={TITLE_HINT}
        error={fieldErrors.title}
      />
      <Textarea
        label="Descripción"
        name="description"
        value={values.description}
        onChange={(event) => updateField("description", event.target.value)}
        hint={DESCRIPTION_HINT}
        error={fieldErrors.description}
      />
      <FormField label="Prioridad" error={fieldErrors.priority}>
        {(control) => (
          <select
            {...control}
            name="priority"
            className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
            value={values.priority}
            onChange={(event) => updateField("priority", event.target.value)}
          >
            <option value="">Elegí una prioridad</option>
            {Object.entries(STORY_PRIORITY_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        )}
      </FormField>
      {!story && (
        <FormField label="Story Points" error={fieldErrors.storyPoints}>
          {(control) => (
            <select
              {...control}
              name="storyPoints"
              className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
              value={values.storyPoints}
              onChange={(event) =>
                updateField("storyPoints", event.target.value)
              }
            >
              <option value="">{UNESTIMATED_LABEL}</option>
              {STORY_POINTS_SCALE.map((points) => (
                <option key={points} value={String(points)}>
                  {points}
                </option>
              ))}
            </select>
          )}
        </FormField>
      )}
      <AcceptanceCriteriaEditor
        items={criteria}
        errors={criterionErrors}
        onChange={setCriteria}
      />
      <Button type="submit" disabled={isSubmitting}>
        {story ? "Guardar cambios" : "Crear historia"}
      </Button>
    </form>
  );
}
