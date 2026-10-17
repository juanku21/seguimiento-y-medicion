import { useId, type TextareaHTMLAttributes } from "react";

type TextareaProps = Omit<
  TextareaHTMLAttributes<HTMLTextAreaElement>,
  "id" | "aria-describedby" | "aria-invalid"
> & {
  label: string;
  hint?: string;
  error?: string;
};

// Campo de texto de varias líneas con su etiqueta, con el mismo comportamiento que Input: la
// indicación y el error se asocian con aria-describedby y un campo con error lleva aria-invalid.
export function Textarea({
  label,
  hint,
  error,
  className = "",
  ...props
}: TextareaProps) {
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
      <textarea
        id={id}
        aria-describedby={describedBy || undefined}
        aria-invalid={error ? true : undefined}
        className={`min-h-24 rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-zinc-900 aria-invalid:border-red-600 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 ${className}`}
        {...props}
      />
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
