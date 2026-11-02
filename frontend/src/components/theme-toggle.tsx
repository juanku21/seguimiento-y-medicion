"use client";

import { Moon, Sun } from "lucide-react";
import { useSyncExternalStore } from "react";
import { isDarkTheme, subscribeTheme, toggleTheme } from "@/lib/theme";

// En el servidor el tema es desconocido: se reserva el espacio y el botón aparece al hidratar.
function getServerTheme(): null {
  return null;
}

// Selector de tema: muestra la luna en modo claro y el sol en modo oscuro, es decir, el modo al que
// se pasa al pulsarlo.
export function ThemeToggle() {
  const dark = useSyncExternalStore(
    subscribeTheme,
    isDarkTheme,
    getServerTheme,
  );

  if (dark === null) {
    return <span aria-hidden="true" className="size-10 shrink-0" />;
  }
  return (
    <button
      type="button"
      onClick={toggleTheme}
      aria-label={dark ? "Activar modo claro" : "Activar modo oscuro"}
      className="inline-flex size-10 shrink-0 items-center justify-center rounded-md text-foreground-muted transition-colors hover:bg-surface-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
    >
      {dark ? (
        <Sun aria-hidden="true" className="size-5" />
      ) : (
        <Moon aria-hidden="true" className="size-5" />
      )}
    </button>
  );
}
