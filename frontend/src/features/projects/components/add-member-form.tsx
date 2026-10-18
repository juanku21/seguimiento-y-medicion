import type { ProjectMemberView } from "@/types/project";

// Propiedades del formulario de alta: el proyecto y la acción que recibe al integrante agregado.
export type AddMemberFormProps = {
  projectId: string;
  onAdded: (member: ProjectMemberView) => void;
};

// Formulario de alta de un integrante por correo (002/US3). Firma mínima para las pruebas de T046:
// la implementación corresponde a T054.
export function AddMemberForm(props: AddMemberFormProps) {
  void props;
  return null;
}
