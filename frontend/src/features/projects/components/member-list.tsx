"use client";

import { useState } from "react";
import { Alert } from "@/components/alert";
import { removeProjectMember } from "@/features/projects/api";
import { AddMemberForm } from "@/features/projects/components/add-member-form";
import type { ProjectMemberView } from "@/types/project";
import { formatDate } from "@/utils/format-date";

// Propiedades de la lista de integrantes: el proyecto, los integrantes tal como los devuelve la API
// y si quien consulta es el propietario, que es quien ve los correos y las acciones de
// administración (FR-030).
export type MemberListProps = {
  projectId: string;
  members: ProjectMemberView[];
  viewerIsOwner: boolean;
};

// Lista de integrantes del proyecto (002/US3). Guarda los integrantes en su propio estado para
// reflejar las altas y las bajas sin volver a consultar. Solo el propietario ve los correos, el
// formulario de alta y la acción de quitar en las filas de los demás (FR-025, FR-030); quien no es
// propietario solo ve la lista. Las restricciones de dominio, como el proyecto finalizado, las
// resuelve el backend y su mensaje se muestra tal cual llega.
export function MemberList({
  projectId,
  members,
  viewerIsOwner,
}: MemberListProps) {
  const [currentMembers, setCurrentMembers] =
    useState<ProjectMemberView[]>(members);
  const [removeError, setRemoveError] = useState<string | null>(null);
  const [removingUserId, setRemovingUserId] = useState<string | null>(null);

  // Incorpora al integrante agregado reemplazando cualquier fila previa de la misma persona, para
  // que nadie aparezca dos veces (FR-029).
  function handleAdded(member: ProjectMemberView) {
    setCurrentMembers((current) => [
      ...current.filter((item) => item.userId !== member.userId),
      member,
    ]);
  }

  // Quita al integrante indicado; ante un rechazo se muestra el mensaje del backend y la lista no
  // cambia.
  async function handleRemove(userId: string) {
    setRemoveError(null);
    setRemovingUserId(userId);
    const result = await removeProjectMember(projectId, userId);
    setRemovingUserId(null);

    if (!result.ok) {
      setRemoveError(result.error.message);
      return;
    }
    setCurrentMembers((current) =>
      current.filter((item) => item.userId !== userId),
    );
  }

  return (
    <div className="flex flex-col gap-5">
      {removeError && <Alert>{removeError}</Alert>}
      <ul aria-label="Integrantes" className="flex flex-col gap-3">
        {currentMembers.map((member) => (
          <li
            key={member.userId}
            className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
          >
            <div className="flex min-w-0 flex-col gap-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-base font-medium break-words">
                  {member.fullName}
                </span>
                {member.isOwner && (
                  <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                    Propietario
                  </span>
                )}
              </div>
              {viewerIsOwner && member.email && (
                <span className="text-sm break-all text-zinc-600 dark:text-zinc-400">
                  {member.email}
                </span>
              )}
              <span className="text-sm text-zinc-600 dark:text-zinc-400">
                Integrante desde:{" "}
                <time dateTime={member.joinedAt}>
                  {formatDate(member.joinedAt)}
                </time>
              </span>
            </div>
            {viewerIsOwner && !member.isOwner && (
              <button
                type="button"
                onClick={() => handleRemove(member.userId)}
                disabled={removingUserId !== null}
                className="inline-flex h-9 items-center justify-center rounded-md border border-zinc-300 px-3 text-sm font-medium text-zinc-900 transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800"
              >
                Quitar
              </button>
            )}
          </li>
        ))}
      </ul>
      {viewerIsOwner && (
        <AddMemberForm projectId={projectId} onAdded={handleAdded} />
      )}
    </div>
  );
}
