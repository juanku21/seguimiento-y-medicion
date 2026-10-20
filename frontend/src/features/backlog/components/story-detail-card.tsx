import type { ReadinessMissing, StoryDetail } from "@/types/story";
import { formatInstant } from "@/utils/format-date";
import {
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
} from "@/utils/story-labels";

// Propiedades de la tarjeta de detalle: la historia completa tal como la devuelve la API.
export type StoryDetailCardProps = {
  story: StoryDetail;
};

// Explicación en español de cada condición que le falta a la historia para quedar lista para
// planificar (FR-029).
const MISSING_LABELS: Record<ReadinessMissing, string> = {
  story_points: "Estimarla en Story Points.",
  acceptance_criteria: "Cargar al menos un criterio de aceptación.",
};

// Arma los Story Points de la historia; null es "Sin estimar" y el 0 es una estimación válida
// (FR-014, FR-015).
function formatStoryPoints(points: StoryDetail["storyPoints"]): string {
  return points === null ? "Sin estimar" : `${points} Story Points`;
}

// Tarjeta con el detalle de una historia (003/US3). Es de presentación: muestra los datos recibidos
// tal cual, con la prioridad y el estado en español, los criterios en el orden en que se cargaron y
// la condición de lista para planificar con lo que falta, que calcula el backend (FR-039, FR-040).
export function StoryDetailCard({ story }: StoryDetailCardProps) {
  const { readiness } = story;

  return (
    <article className="flex flex-col gap-6 rounded-md border border-zinc-300 px-5 py-5 dark:border-zinc-700">
      {/* Encabezado: título, condición de lista para planificar y, si corresponde, la asignación a
          un sprint. */}
      <header className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-xl font-semibold break-words">{story.title}</h2>
          <span
            className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
              readiness.ready
                ? "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200"
                : "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
            }`}
          >
            {readiness.ready
              ? "Lista para planificar"
              : "No lista para planificar"}
          </span>
        </div>
        {story.everAssignedToSprint && (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Está o estuvo asignada a un sprint.
          </p>
        )}
        {story.description !== null && (
          <p className="text-sm break-words whitespace-pre-line">
            {story.description}
          </p>
        )}
      </header>

      {/* Resumen con el mismo formato de la fila del listado: prioridad, estimación y estado
          separados por puntos medios. */}
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        {STORY_PRIORITY_LABELS[story.priority]} ·{" "}
        {formatStoryPoints(story.storyPoints)} ·{" "}
        {STORY_STATUS_LABELS[story.status]}
      </p>

      {/* Autor y fechas de creación y de última actualización; son instantes UTC y se muestran con
          el día que corresponde en la zona horaria del navegador. */}
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
        <dt className="text-zinc-600 dark:text-zinc-400">Creada por</dt>
        <dd>{story.createdBy.fullName}</dd>
        <dt className="text-zinc-600 dark:text-zinc-400">Creada el</dt>
        <dd>
          <time dateTime={story.createdAt}>
            {formatInstant(
              story.createdAt,
              Intl.DateTimeFormat().resolvedOptions().timeZone,
            )}
          </time>
        </dd>
        <dt className="text-zinc-600 dark:text-zinc-400">Actualizada el</dt>
        <dd>
          <time dateTime={story.updatedAt}>
            {formatInstant(
              story.updatedAt,
              Intl.DateTimeFormat().resolvedOptions().timeZone,
            )}
          </time>
        </dd>
      </dl>

      {/* Criterios de aceptación en el orden recibido; sin criterios la lista queda vacía y se
          avisa con un texto aparte. */}
      <section className="flex flex-col gap-2">
        <h3 className="text-base font-medium">Criterios de aceptación</h3>
        <ol
          aria-label="Criterios de aceptación"
          className="flex list-decimal flex-col gap-1 pl-5 text-sm"
        >
          {story.acceptanceCriteria.map((criterion, index) => (
            <li key={index} className="break-words">
              {criterion}
            </li>
          ))}
        </ol>
        {story.acceptanceCriteria.length === 0 && (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Todavía no tiene criterios cargados.
          </p>
        )}
      </section>

      {/* Lo que falta para planificar, solo cuando la historia todavía no está lista. */}
      {!readiness.ready && (
        <section className="flex flex-col gap-2">
          <h3 className="text-base font-medium">Qué falta para planificar</h3>
          <ul
            aria-label="Qué falta para planificar"
            className="flex list-disc flex-col gap-1 pl-5 text-sm"
          >
            {readiness.missing.map((reason) => (
              <li key={reason}>{MISSING_LABELS[reason]}</li>
            ))}
          </ul>
        </section>
      )}
    </article>
  );
}
