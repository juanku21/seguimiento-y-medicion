import type { TextareaHTMLAttributes } from "react";
import {
  FIELD_CONTROL_CLASS_NAME,
  FormField,
  type FieldControlAttributes,
  type FieldProps,
} from "@/components/form-field";

type TextareaProps = Omit<
  TextareaHTMLAttributes<HTMLTextAreaElement>,
  FieldControlAttributes
> &
  FieldProps;

// Campo de texto de varias líneas con su etiqueta; la indicación y el error los resuelve FormField.
export function Textarea({
  label,
  hint,
  error,
  className = "",
  ...props
}: TextareaProps) {
  return (
    <FormField label={label} hint={hint} error={error}>
      {(control) => (
        <textarea
          {...control}
          className={`min-h-24 py-2 ${FIELD_CONTROL_CLASS_NAME} ${className}`}
          {...props}
        />
      )}
    </FormField>
  );
}
