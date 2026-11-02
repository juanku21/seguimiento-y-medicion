import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";

type DefectSelectFieldProps = {
  label: string;
  name: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  error?: string;
  disabled?: boolean;
};

// Select nativo con su etiqueta y los estilos comunes de los campos; su primera opción es vacía.
// Lo comparten el alta y la corrección de un defecto.
export function DefectSelectField({
  label,
  name,
  value,
  onChange,
  options,
  error,
  disabled,
}: DefectSelectFieldProps) {
  return (
    <FormField label={label} error={error}>
      {(control) => (
        <select
          {...control}
          name={name}
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
          className={`h-11 ${FIELD_CONTROL_CLASS_NAME} disabled:opacity-60`}
        >
          <option value=""></option>
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      )}
    </FormField>
  );
}
