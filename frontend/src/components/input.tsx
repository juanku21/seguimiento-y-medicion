import type { InputHTMLAttributes } from "react";
import {
  FIELD_CONTROL_CLASS_NAME,
  FormField,
  type FieldControlAttributes,
  type FieldProps,
} from "@/components/form-field";

type InputProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  FieldControlAttributes
> &
  FieldProps;

// Campo de texto de una línea con su etiqueta; la indicación y el error los resuelve FormField.
export function Input({
  label,
  hint,
  error,
  className = "",
  ...props
}: InputProps) {
  return (
    <FormField label={label} hint={hint} error={error}>
      {(control) => (
        <input
          {...control}
          className={`h-11 ${FIELD_CONTROL_CLASS_NAME} ${className}`}
          {...props}
        />
      )}
    </FormField>
  );
}
