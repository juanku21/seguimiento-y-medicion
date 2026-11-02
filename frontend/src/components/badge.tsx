import type { ReactNode } from "react";

// Tono de una insignia: siempre acompaña a un texto, nunca comunica solo con el color.
export type BadgeTone =
  "neutral" | "primary" | "success" | "warning" | "danger";

const TONE_CLASS_NAMES: Record<BadgeTone, string> = {
  neutral: "bg-surface-muted text-foreground-muted",
  primary: "bg-primary-soft text-primary",
  success: "bg-success-soft text-success",
  warning: "bg-warning-soft text-warning",
  danger: "bg-danger-soft text-danger",
};

// Insignia para estados y categorías.
export function Badge({
  tone = "neutral",
  children,
}: {
  tone?: BadgeTone;
  children: ReactNode;
}) {
  return (
    <span
      className={`inline-flex shrink-0 items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${TONE_CLASS_NAMES[tone]}`}
    >
      {children}
    </span>
  );
}
