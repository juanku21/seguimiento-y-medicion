"use client";

import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { addProjectMember } from "@/features/projects/api";
import type { ProjectMemberView } from "@/types/project";

// Propiedades del formulario de alta: el proyecto y la acción que recibe al integrante agregado.
export type AddMemberFormProps = {
  projectId: string;
  onAdded: (member: ProjectMemberView) => void;
};

// Formulario de alta de un integrante por correo (002/US3). El correo se envía tal como se
// escribió: la normalización y las reglas de negocio las resuelve el backend (FR-020 a FR-022), y
// sus rechazos se muestran con el mensaje recibido.
export function AddMemberForm({ projectId, onAdded }: AddMemberFormProps) {
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Envía el alta. La referencia descarta un segundo envío mientras el primero sigue en curso; al
  // agregarse la persona se limpia el campo y se entrega el integrante recibido.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await addProjectMember(projectId, email);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (!result.ok) {
      setError(result.error.message);
      return;
    }
    setEmail("");
    onAdded(result.data);
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-4">
      {error && <Alert>{error}</Alert>}
      <Input
        label="Correo electrónico"
        name="email"
        type="email"
        autoComplete="off"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
      />
      <Button type="submit" disabled={isSubmitting}>
        Agregar integrante
      </Button>
    </form>
  );
}
