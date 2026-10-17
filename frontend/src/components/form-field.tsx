import { useId, type ReactNode } from "react";

// Atributos que FormField asigna al control; los campos que lo usan los excluyen de sus props.
export type FieldControlAttributes = "id" | "aria-describedby" | "aria-invalid";

export type FieldProps = {
  label: string;
  hint?: string;
  error?: string;
};

type FieldControlProps = {
  id: string;
  "aria-describedby"?: string;
  "aria-invalid"?: true;
};

type FormFieldProps = FieldProps & {
  children: (control: FieldControlProps) => ReactNode;
};

// Estilos comunes a los controles de texto, que cada campo completa con su alto y su relleno.
export const FIELD_CONTROL_CLASS_NAME =
  "rounded-md border border-zinc-300 bg-white px-3 text-sm text-zinc-900 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-zinc-900 aria-invalid:border-red-600 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100";

// Etiqueta, indicación y error de un campo de formulario. Entrega al control los atributos que lo
// asocian con la indicación y el error mediante aria-describedby, y lo marca con aria-invalid
// cuando tiene error, para las tecnologías de apoyo.
export function FormField({ label, hint, error, children }: FormFieldProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = [hint && hintId, error && errorId]
    .filter(Boolean)
    .join(" ");

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children({
        id,
        "aria-describedby": describedBy || undefined,
        "aria-invalid": error ? true : undefined,
      })}
      {hint && (
        <p id={hintId} className="text-xs text-zinc-600 dark:text-zinc-400">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-xs text-red-700 dark:text-red-400">
          {error}
        </p>
      )}
    </div>
  );
}
