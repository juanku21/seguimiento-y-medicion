"use client";

import { useEffect, useState, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { listProjectDefects } from "@/features/defects/api";
import { DefectRow } from "@/features/defects/components/defect-row";
import { useRequireSession } from "@/hooks/use-session";
import type { ApiError } from "@/lib/api-client";
import type { DefectFilterValues, ProjectDefects } from "@/types/defect";
import { DEFECT_EMPTY_REASON_LABELS } from "@/utils/defect-labels";

// Propiedades del listado: el proyecto consultado y los filtros elegidos.
export type DefectListProps = {
  projectId: string;
  filters: DefectFilterValues;
};

// Estado de la consulta: todavía sin respuesta, la lista recibida o el error con su detalle por
// campo.
type ListState =
  | { status: "loading" }
  | { status: "loaded"; data: ProjectDefects }
  | { status: "error"; error: ApiError };

// Listado de defectos del proyecto (007/US3): vuelve a pedir la lista cada vez que cambian el
// proyecto o los filtros y muestra una fila por defecto en el orden recibido. Los vacíos se
// informan con el aviso de su motivo y sin presentarse como error (FR-045); un rechazo del backend
// se muestra con su mensaje y el detalle por campo.
export function DefectList({
  projectId,
  filters,
}: DefectListProps): ReactElement {
  const token = useRequireSession();
  const [state, setState] = useState<ListState>({ status: "loading" });

  // Pide la lista con un token vigente y cada vez que cambian el proyecto o los filtros. La bandera
  // descarta la respuesta si llegó otra consulta o el componente se desmontó.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listProjectDefects(projectId, filters).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", data: result.data }
          : { status: "error", error: result.error },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, projectId, filters]);

  if (state.status === "error") {
    return (
      <Alert>
        <p>{state.error.message}</p>
        {state.error.fields.map((field) => (
          <p key={field.field}>{field.message}</p>
        ))}
      </Alert>
    );
  }
  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando defectos…
      </p>
    );
  }

  const { defects, emptyReason } = state.data;
  if (defects.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400">
        {DEFECT_EMPTY_REASON_LABELS[emptyReason ?? "no_defects"]}
      </p>
    );
  }
  return (
    <ul aria-label="Defectos del proyecto" className="flex flex-col gap-3">
      {defects.map((defect) => (
        <DefectRow key={defect.id} defect={defect} />
      ))}
    </ul>
  );
}
