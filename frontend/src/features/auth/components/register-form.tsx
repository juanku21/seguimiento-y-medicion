"use client";

import Link from "next/link";
import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { registerUser } from "@/features/auth/api";
import {
  validateEmail,
  validateFullName,
  validatePassword,
} from "@/features/auth/validation";
import type { FieldError, RegisterRequest } from "@/types/auth";
import { normalizeEmail } from "@/utils/normalize-email";

type FieldName = keyof RegisterRequest;
type FieldErrors = Partial<Record<FieldName, string>>;

// Indicación del límite de la contraseña, visible desde el inicio y no recién al fallar (RC-02).
const PASSWORD_HINT =
  "Entre 8 y 72 caracteres, con al menos una letra y un número. Las letras acentuadas y los emojis ocupan más espacio: en total no puede superar los 72 bytes.";

// Indica si un nombre de campo informado por el backend corresponde a un campo del formulario.
function isFieldName(value: string): value is FieldName {
  return value === "fullName" || value === "email" || value === "password";
}

// Aplica las reglas de RC-02 a los valores ya normalizados y devuelve solo los campos con error.
function validate(values: RegisterRequest): FieldErrors {
  const errors: FieldErrors = {};
  const checks: [FieldName, string | null][] = [
    ["fullName", validateFullName(values.fullName)],
    ["email", validateEmail(values.email)],
    ["password", validatePassword(values.password)],
  ];
  for (const [field, message] of checks) {
    if (message) {
      errors[field] = message;
    }
  }
  return errors;
}

// Ubica junto a cada campo el detalle que informa el backend, descartando los que no son del
// formulario.
function toFieldErrors(fields: FieldError[]): FieldErrors {
  const errors: FieldErrors = {};
  for (const { field, message } of fields) {
    if (isFieldName(field)) {
      errors[field] = message;
    }
  }
  return errors;
}

// Formulario de registro de cuenta (US1). Valida antes de enviar, envía el correo normalizado y,
// al confirmarse el alta, ofrece ir a iniciar sesión sin guardar nada en el navegador (FR-009).
export function RegisterForm() {
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isRegistered, setIsRegistered] = useState(false);
  const submittingRef = useRef(false);

  // Envía el registro. La referencia descarta un segundo envío mientras el primero sigue en curso,
  // aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const values: RegisterRequest = {
      fullName,
      email: normalizeEmail(email),
      password,
    };
    const errors = validate(values);
    setFieldErrors(errors);
    setGeneralError(null);
    if (Object.keys(errors).length > 0) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await registerUser(values);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setIsRegistered(true);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo.
    setFieldErrors(toFieldErrors(result.error.fields));
    setGeneralError(result.error.message);
  }

  // Confirmación del alta: el registro no inicia sesión, así que se ofrece el enlace para hacerlo.
  if (isRegistered) {
    return (
      <div className="flex flex-col gap-4">
        <Alert variant="success">
          <p>Cuenta creada. Ya podés ingresar con tu correo y tu contraseña.</p>
        </Alert>
        <Link
          href="/login"
          className="text-sm font-medium underline underline-offset-4"
        >
          Iniciar sesión
        </Link>
      </div>
    );
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {generalError && <Alert>{generalError}</Alert>}
      <Input
        label="Nombre completo"
        name="fullName"
        autoComplete="name"
        value={fullName}
        onChange={(event) => setFullName(event.target.value)}
        error={fieldErrors.fullName}
      />
      <Input
        label="Correo electrónico"
        name="email"
        type="email"
        autoComplete="email"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        error={fieldErrors.email}
      />
      <Input
        label="Contraseña"
        name="password"
        type="password"
        autoComplete="new-password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        hint={PASSWORD_HINT}
        error={fieldErrors.password}
      />
      <Button type="submit" disabled={isSubmitting}>
        Crear cuenta
      </Button>
    </form>
  );
}
