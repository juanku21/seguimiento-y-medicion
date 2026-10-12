"use client";

import { useRouter } from "next/navigation";
import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { loginUser } from "@/features/auth/api";
import { useSession } from "@/hooks/use-session";
import { normalizeEmail } from "@/utils/normalize-email";

// Ruta del área protegida a la que se llega después de iniciar sesión.
const PROTECTED_HOME = "/profile";

// Formulario de inicio de sesión (US2). Envía el correo normalizado; si el backend emite la sesión
// guarda el token y lleva al área protegida, y si la rechaza muestra el mensaje que devuelve, que
// es el mismo para correo inexistente y contraseña incorrecta (FR-014).
export function LoginForm() {
  const router = useRouter();
  const { startSession } = useSession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Envía las credenciales. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await loginUser({ email: normalizeEmail(email), password });
    submittingRef.current = false;
    setIsSubmitting(false);

    if (!result.ok) {
      setError(result.error.message);
      return;
    }
    startSession(result.data.token);
    router.push(PROTECTED_HOME);
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {error && <Alert>{error}</Alert>}
      <Input
        label="Correo electrónico"
        name="email"
        type="email"
        autoComplete="email"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
      />
      <Input
        label="Contraseña"
        name="password"
        type="password"
        autoComplete="current-password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
      />
      <Button type="submit" disabled={isSubmitting}>
        Iniciar sesión
      </Button>
    </form>
  );
}
