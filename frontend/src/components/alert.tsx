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
          ? "border-red-300 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200"
          : "border-green-300 bg-green-50 text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200"
      }`}
    >
      {children}
    </div>
  );
}
