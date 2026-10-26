"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Alert } from "@/components/alert";
import { listProjectEffort } from "@/features/effort/api";
import {
  EffortFilters,
  type SprintOption,
} from "@/features/effort/components/effort-filters";
import { getProjectDetail } from "@/features/projects/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { useRequireSession } from "@/hooks/use-session";
import type { ApiError, ApiResult } from "@/lib/api-client";
import type {
  EffortEntry,
  ProjectEffort,
  ProjectEffortFilter,
  StoryRef,
  UserRef,
} from "@/types/effort";
import { formatHours } from "@/utils/effort-format";
import { formatDate } from "@/utils/format-date";

// Propiedades de la vista del esfuerzo del proyecto: el proyecto tomado de la ruta.
export type ProjectEffortViewProps = {
  projectId: string;
};

// Opciones de los selectores de filtro: integrantes, historias y sprints.
type FilterChoices = {
  members: UserRef[];
  stories: StoryRef[];
  sprints: SprintOption[];
};

// Estado de la consulta del esfuerzo: todavía sin respuesta, el esfuerzo recibido o el error
// devuelto, con su detalle por campo.
type EffortState =
  | { status: "loading" }
  | { status: "loaded"; data: ProjectEffort }
  | { status: "error"; error: ApiError };

// Filtros iniciales: ninguno aplicado.
const NO_FILTERS: ProjectEffortFilter = {
  userId: "",
  storyId: "",
  sprintId: "",
  from: "",
  to: "",
};

const EMPTY_MESSAGE_CLASS_NAME =
  "rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400";

// Agrega a la lista los elementos que todavía no figuran, comparando por id y conservando el orden
// de aparición.
function appendUnique<T extends { id: string }>(list: T[], items: T[]): T[] {
  const result = [...list];
  items.forEach((item) => {
    if (!result.some((current) => current.id === item.id)) {
      result.push(item);
    }
  });
  return result;
}

// Arma las opciones de los filtros: los integrantes vigentes del proyecto (002) seguidos de los
// autores de registros que ya no son integrantes (FR-040, research.md sección 14), y las historias y
// los sprints que aparecen en los registros del proyecto, sin repetir.
function filterChoices(
  members: UserRef[],
  entries: EffortEntry[],
): FilterChoices {
  return {
    members: appendUnique(
      members,
      entries.map((entry) => entry.author),
    ),
    stories: appendUnique<StoryRef>(
      [],
      entries.map((entry) => entry.story),
    ),
    sprints: appendUnique<SprintOption>(
      [],
      entries.flatMap((entry) => (entry.sprint ? [entry.sprint] : [])),
    ),
  };
}

// Pide el estado del proyecto y el esfuerzo sin filtros para armar las opciones de los filtros; si
// alguna de las dos consultas falla, devuelve su error.
async function loadFilterChoices(
  projectId: string,
): Promise<ApiResult<FilterChoices>> {
  const [project, effort] = await Promise.all([
    getProjectDetail(projectId),
    listProjectEffort(projectId, NO_FILTERS),
  ]);
  if (!project.ok) {
    return project;
  }
  if (!effort.ok) {
    return effort;
  }
  const members = project.data.members.map((member) => ({
    id: member.userId,
    fullName: member.fullName,
  }));
  return { ok: true, data: filterChoices(members, effort.data.entries) };
}

// Esfuerzo del proyecto con filtros, lista y total (006/US4). Arma las opciones de los filtros una
// vez por proyecto y vuelve a pedir el esfuerzo cada vez que cambian los filtros: el filtrado, el
// orden, el total y el motivo del vacío los resuelve el backend (FR-036 a FR-045). Los vacíos se
// informan sin presentarse como error (FR-042); un filtro rechazado se muestra con su mensaje y el
// detalle por campo (FR-043).
export function ProjectEffortView({
  projectId,
}: ProjectEffortViewProps): ReactNode {
  const token = useRequireSession();
  const [filters, setFilters] = useState<ProjectEffortFilter>(NO_FILTERS);
  const [effort, setEffort] = useState<EffortState>({ status: "loading" });
  const loadChoices = useCallback(
    () => loadFilterChoices(projectId),
    [projectId],
  );
  const choices = useAuthorizedQuery(loadChoices);

  // Pide el esfuerzo con un token vigente y cada vez que cambian el proyecto o los filtros. La
  // bandera descarta la respuesta si llegó otra consulta o el componente se desmontó.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listProjectEffort(projectId, filters).then((result) => {
      if (!isCurrent) {
        return;
      }
      setEffort(
        result.ok
          ? { status: "loaded", data: result.data }
          : { status: "error", error: result.error },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, projectId, filters]);

  // Sin las opciones (por ejemplo, un proyecto ajeno o inexistente) no hay vista que mostrar.
  if (choices.status === "error") {
    return <Alert>{choices.message}</Alert>;
  }
  const options: FilterChoices =
    choices.status === "loaded"
      ? choices.data
      : { members: [], stories: [], sprints: [] };

  return (
    <div className="flex flex-col gap-6">
      <EffortFilters
        value={filters}
        onChange={setFilters}
        members={options.members}
        stories={options.stories}
        sprints={options.sprints}
      />
      <EffortResults effort={effort} />
    </div>
  );
}

// Resultado de la consulta: el error con su detalle por campo, el aviso de carga, o el total con la
// lista de registros o el aviso de vacío que corresponda.
function EffortResults({ effort }: { effort: EffortState }): ReactNode {
  if (effort.status === "error") {
    return (
      <Alert>
        <p>{effort.error.message}</p>
        {effort.error.fields.map((field) => (
          <p key={field.field}>{field.message}</p>
        ))}
      </Alert>
    );
  }
  if (effort.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando esfuerzo…
      </p>
    );
  }

  const { entries, totalHours, emptyReason } = effort.data;
  return (
    <section className="flex flex-col gap-4">
      <p className="text-base font-semibold tabular-nums">
        {`Total: ${formatHours(totalHours)}`}
      </p>

      {/* Vacíos: el proyecto sin esfuerzo y los filtros sin coincidencias tienen avisos distintos y
          ninguno es un error (FR-042). */}
      {entries.length === 0 ? (
        <p className={EMPTY_MESSAGE_CLASS_NAME}>
          {emptyReason === "no_effort"
            ? "Todavía no hay esfuerzo registrado en el proyecto."
            : "Ningún registro coincide con los filtros."}
        </p>
      ) : (
        <ul
          aria-label="Registros de esfuerzo del proyecto"
          className="flex flex-col gap-3"
        >
          {entries.map((entry) => (
            <ProjectEffortRow key={entry.id} entry={entry} />
          ))}
        </ul>
      )}
    </section>
  );
}

// Fila de solo lectura de un registro del proyecto: fecha, horas, actividad, autor, historia y
// sprint, o "Sin sprint" si no tiene.
function ProjectEffortRow({ entry }: { entry: EffortEntry }): ReactNode {
  return (
    <li className="flex flex-col gap-2 rounded-md border border-zinc-300 px-4 py-3 text-sm dark:border-zinc-700">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="tabular-nums">{formatDate(entry.entryDate)}</span>
        <span className="font-medium tabular-nums">
          {formatHours(entry.hours)}
        </span>
      </div>
      <span className="break-words">{entry.activity}</span>
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-zinc-600 dark:text-zinc-400">
        <span>{entry.author.fullName}</span>
        <span className="break-words">{entry.story.title}</span>
        <span>{entry.sprint ? entry.sprint.name : "Sin sprint"}</span>
      </div>
    </li>
  );
}
