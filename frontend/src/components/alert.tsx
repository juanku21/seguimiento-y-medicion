import type { ReactNode } from "react";

type AlertProps = {
  variant?: "error" | "success";
  children: ReactNode;
};

// Mensaje general de la interfaz. Los errores usan role="alert" para que los lectores de pantalla
// los anuncien de inmediato; las confirmaciones usan role="status", que se anuncia sin interrumpir.
export function Alert({ variant = "error", children }: AlertProps) {
  const isError = variant === "error";
  return (
    <div
      role={isError ? "alert" : "status"}
      className={`rounded-md border px-4 py-3 text-sm ${
        isError
          ? "border-danger/40 bg-danger-soft text-danger"
          : "border-success/40 bg-success-soft text-success"
      }`}
    >
      {children}
    </div>
  );
}
