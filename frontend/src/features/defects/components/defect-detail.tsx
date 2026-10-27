"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Alert } from "@/components/alert";
import { getDefect } from "@/features/defects/api";
import { DefectStatusActions } from "@/features/defects/components/defect-status-actions";
import { useRequireSession } from "@/hooks/use-session";
import type { Defect } from "@/types/defect";
import { formatDate } from "@/utils/format-date";
import {
  DEFECT_SEVERITY_LABELS,
  DEFECT_STATUS_LABELS,
} from "@/utils/defect-labels";

// Propiedades del detalle: el identificador del defecto tomado de la ruta.
export type DefectDetailProps = {
  defectId: string;
};

// Estado de la consulta: todavía sin respuesta, el defecto recibido o el mensaje de error.
type DetailState =
  | { status: "loading" }
  | { status: "loaded"; defect: Defect }
  | { status: "error"; message: string };

// Detalle de un defecto (007/US2): pide el defecto con un token vigente y muestra su descripción,
// severidad, estado, historia, sprints de detección y de resolución, autoría y último cambio de
// estado, junto con las acciones de estado. Cada cambio reemplaza el defecto mostrado por el que
// entregan las acciones. El defecto inexistente o ajeno llega como 404 y se muestra solo su mensaje;
// ante un 401 el cliente HTTP borra el token y la guardia lleva a /login.
export function DefectDetail({ defectId }: DefectDetailProps) {
  const token = useRequireSession();
  const [state, setState] = useState<DetailState>({ status: "loading" });

  // Pide el detalle cada vez que hay un token vigente o cambia el defecto. La bandera descarta la
  // respuesta si el componente se desmontó o el pedido quedó viejo antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    getDefect(defectId).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", defect: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, defectId]);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando defecto…
      </p>
    );
  }

  const { defect } = state;

  return (
    <div className="flex flex-col gap-8">
      {/* Datos del defecto: estado y severidad, descripción y la lista de vínculos y autoría. */}
      <section className="flex flex-col gap-3">
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          {DEFECT_STATUS_LABELS[defect.status]} · Severidad{" "}
          {DEFECT_SEVERITY_LABELS[defect.severity]}
        </p>
        <p className="text-sm whitespace-pre-wrap">{defect.description}</p>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="font-medium">Historia</dt>
          <dd>
            <Link
              href={`/projects/${encodeURIComponent(defect.projectId)}/backlog/${encodeURIComponent(defect.story.id)}`}
              className="underline underline-offset-4"
            >
              {defect.story.title}
            </Link>
          </dd>
          <dt className="font-medium">Sprint de detección</dt>
          <dd>{defect.detectedSprint.name}</dd>
          <dt className="font-medium">Sprint de resolución</dt>
          <dd>{defect.resolvedSprint?.name ?? "Sin resolver"}</dd>
          <dt className="font-medium">Registrado por</dt>
          <dd>
            {defect.createdBy.fullName} el {formatDate(defect.createdAt)}
          </dd>
          <dt className="font-medium">Último cambio de estado</dt>
          <dd>
            {defect.statusChangedBy.fullName} el{" "}
            {formatDate(defect.statusChangedAt)}
          </dd>
        </dl>
      </section>

      <DefectStatusActions
        defect={defect}
        onChanged={(changed) => setState({ status: "loaded", defect: changed })}
      />
    </div>
  );
}
