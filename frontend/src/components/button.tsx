import type { ButtonHTMLAttributes } from "react";

// Botón principal de la interfaz. Acepta los atributos nativos de <button>; por defecto es de tipo
// "button" para no enviar formularios por accidente y se atenúa mientras está deshabilitado.
export function Button({
  type = "button",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type={type}
      className={`inline-flex h-11 w-full items-center justify-center rounded-md bg-zinc-900 px-4 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300 ${className}`}
      {...props}
    />
  );
}
