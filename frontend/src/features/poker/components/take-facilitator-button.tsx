import { useEffect, useState, type ReactElement } from "react";

// Propiedades del botón para tomar el rol de facilitador: el momento a partir del cual el rol queda
// disponible según la instantánea (null si el facilitador está conectado) y la acción que pide el
// rol.
export type TakeFacilitatorButtonProps = {
  facilitatorAvailableAt: string | null;
  onTake: () => void;
};

// Formatea una espera en milisegundos como m:ss, redondeando los segundos hacia arriba para que el
// último segundo se muestre como 0:01 y no como 0:00.
function formatRemaining(remainingMs: number): string {
  const totalSeconds = Math.ceil(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

// Botón "Tomar el rol" de la sala de Planning Poker (005/US6). Es de presentación: si el rol se
// puede tomar lo resuelve el backend (FR-068, FR-069). Con el facilitador conectado no muestra nada;
// con facilitatorAvailableAt vencido ofrece el botón, que pide el rol con onTake; antes muestra
// cuánto falta en m:ss. La cuenta se actualiza sola cada segundo y, al vencer, aparece el botón sin
// esperar una instantánea nueva.
export function TakeFacilitatorButton({
  facilitatorAvailableAt,
  onTake,
}: TakeFacilitatorButtonProps): ReactElement | null {
  const [now, setNow] = useState(() => Date.now());
  const availableAtMs =
    facilitatorAvailableAt === null ? null : Date.parse(facilitatorAvailableAt);
  const remainingMs = availableAtMs === null ? null : availableAtMs - now;

  // Con el facilitador desconectado vuelve a leer el reloj cada segundo y deja de hacerlo al vencer
  // la espera, cuando ya se ofrece el botón. La primera lectura también corrige un reloj que quedó
  // viejo si facilitatorAvailableAt cambió.
  useEffect(() => {
    if (availableAtMs === null) {
      return;
    }
    const timer = setInterval(() => {
      const current = Date.now();
      setNow(current);
      if (current >= availableAtMs) {
        clearInterval(timer);
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [availableAtMs]);

  if (remainingMs === null) {
    return null;
  }
  if (remainingMs > 0) {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        El rol de facilitador queda disponible en {formatRemaining(remainingMs)}
        .
      </p>
    );
  }
  return (
    <button
      type="button"
      onClick={onTake}
      className="self-start rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300 dark:focus-visible:outline-zinc-100"
    >
      Tomar el rol
    </button>
  );
}
