import { LoaderCircle } from "lucide-react";
import type { ReactNode } from "react";

// Aviso de carga: los lectores de pantalla lo anuncian sin interrumpir y el ícono se detiene si la
// persona pidió reducir el movimiento.
export function LoadingState({ children }: { children: ReactNode }) {
  return (
    <p
      role="status"
      className="flex items-center gap-2 py-2 text-sm text-foreground-muted"
    >
      <LoaderCircle
        aria-hidden="true"
        className="size-4 shrink-0 animate-spin motion-reduce:animate-none"
      />
      {children}
    </p>
  );
}
