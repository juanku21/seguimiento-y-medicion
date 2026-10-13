"use client";

import { useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { logoutUser } from "@/features/auth/api";
import { useSession } from "@/hooks/use-session";

// Botón de cierre de sesión (001/US5). Pide a la API que revoque la sesión de esta pestaña y, ante
// el 204, descarta el token para que la interfaz vuelva al estado de visitante. Si la API responde
// 401, el cliente HTTP ya descartó la sesión; ante cualquier otro error conserva la sesión y
// muestra el mensaje recibido.
export function LogoutButton() {
  const { endSession } = useSession();
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Envía el cierre y resuelve el resultado; el botón queda deshabilitado mientras espera.
  async function handleClick() {
    setIsSubmitting(true);
    setError(null);
    const result = await logoutUser();
    setIsSubmitting(false);

    if (result.ok) {
      endSession();
      return;
    }
    if (result.error.status !== 401) {
      setError(result.error.message);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {error && <Alert>{error}</Alert>}
      <Button onClick={handleClick} disabled={isSubmitting}>
        Cerrar sesión
      </Button>
    </div>
  );
}
