import type { ButtonHTMLAttributes } from "react";

// Variantes de acción: una sola primaria por vista; la de peligro, para lo irreversible.
export type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";

const VARIANT_CLASS_NAMES: Record<ButtonVariant, string> = {
  primary: "bg-primary text-on-primary hover:bg-primary-hover",
  secondary:
    "border border-border-strong bg-surface text-foreground hover:bg-surface-muted",
  danger:
    "border border-danger bg-surface text-danger hover:bg-danger-soft focus-visible:outline-danger",
  ghost: "text-foreground-muted hover:bg-surface-muted hover:text-foreground",
};

// Clases de una acción, compartidas con los enlaces que se presentan como botones.
export function buttonClassName(
  variant: ButtonVariant = "primary",
  fullWidth = false,
): string {
  const width = fullWidth ? "w-full" : "w-full sm:w-auto sm:self-start";
  return `inline-flex min-h-11 items-center justify-center gap-2 rounded-md px-4 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:cursor-not-allowed disabled:opacity-60 sm:min-h-10 ${width} ${VARIANT_CLASS_NAMES[variant]}`;
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  fullWidth?: boolean;
};

// Botón de la interfaz. Acepta los atributos nativos de <button>; por defecto es de tipo "button"
// para no enviar formularios por accidente. Ocupa todo el ancho en móvil y el de su texto desde sm.
export function Button({
  type = "button",
  variant = "primary",
  fullWidth = false,
  className = "",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={`${buttonClassName(variant, fullWidth)} ${className}`}
      {...props}
    />
  );
}
