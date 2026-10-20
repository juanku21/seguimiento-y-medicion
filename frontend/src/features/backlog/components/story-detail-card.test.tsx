import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render } from "@testing-library/react";
import { getStory } from "@/features/backlog/api";
import { StoryDetailCard } from "@/features/backlog/components/story-detail-card";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";
import type { StoryDetail } from "@/types/story";

// Pruebas del detalle de una historia (T041). La tarjeta es de presentación: recibe el StoryDetail
// que la pantalla (T046) pide con getStory. Por eso los escenarios de datos se verifican montando la
// tarjeta, y los de error (historia ajena e inexistente) atravesando la llamada real y el cliente
// HTTP; solo se sustituye fetch, que es la frontera con el backend. La condición de lista para
// planificar y qué falta los calcula el backend (RN-12): acá se verifica qué muestra la vista.
//
// Contrato que fijan estas pruebas:
// - StoryDetailCard recibe story (StoryDetail) y muestra el título, la descripción, la etiqueta en
//   español de la prioridad y del estado, los Story Points como "N Story Points" (el 0 incluido) o
//   "Sin estimar" si son null, el nombre del autor, y la fecha de creación y la de última
//   actualización (AAAA-MM-DD o DD/MM/AAAA).
// - Los criterios de aceptación se muestran en una lista con aria-label="Criterios de aceptación",
//   un elemento li por criterio y en el orden recibido; sin criterios la lista no tiene elementos.
// - Si está lista muestra "Lista para planificar"; si no, "No lista para planificar" y una lista con
//   aria-label="Qué falta para planificar" con un li por motivo de readiness.missing: el de
//   story_points menciona la estimación o los Story Points y el de acceptance_criteria menciona los
//   criterios de aceptación.
// - Con everAssignedToSprint verdadero muestra que "está o estuvo asignada a un sprint"; si es
//   falso no menciona ninguna asignación a un sprint.
// - getStory(storyId) pide GET /api/v1/stories/{storyId} con la sesión de la pestaña y devuelve
//   ApiResult<StoryDetail>; el 404 llega como error, sin datos de la historia.

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";

// Historia lista para planificar del ejemplo "lista" del contrato (US3-1): prioridad Media,
// 8 Story Points y tres criterios cargados en un orden que no es el alfabético.
const readyStory: StoryDetail = {
  id: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  projectId: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
  title: "Exportar el reporte de velocidad a PDF",
  description: "El equipo necesita compartir la velocidad con la gerencia.",
  priority: "medium",
  status: "pending",
  storyPoints: 8,
  acceptanceCriteria: [
    "El PDF incluye la velocidad de los últimos cinco sprints.",
    "Cada sprint muestra los Story Points comprometidos y completados.",
    "Al pie figura la fecha de generación.",
  ],
  readiness: { ready: true, missing: [] },
  createdBy: {
    id: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
    fullName: "Bruno Gómez",
  },
  createdAt: "2026-10-10T14:30:00Z",
  updatedAt: "2026-10-12T15:45:00Z",
  everAssignedToSprint: false,
};

// Historia recién cargada sin estimar y sin criterios: no está lista y le faltan los dos (US3-2).
const unestimatedStory: StoryDetail = {
  ...readyStory,
  id: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Notificar por correo el cierre del sprint",
  description: null,
  priority: "low",
  storyPoints: null,
  acceptanceCriteria: [],
  readiness: {
    ready: false,
    missing: ["story_points", "acceptance_criteria"],
  },
};

// Monta la tarjeta y devuelve su texto completo.
function renderCardText(story: StoryDetail): string {
  const { container } = render(<StoryDetailCard story={story} />);
  return container.textContent ?? "";
}

// Devuelve el texto de cada elemento de la lista con el aria-label indicado, o null si no existe.
function listItems(label: string): string[] | null {
  const list = document.querySelector(`[aria-label="${label}"]`);
  if (list === null) {
    return null;
  }
  return Array.from(list.querySelectorAll("li")).map(
    (item) => item.textContent ?? "",
  );
}

// Indica si el texto muestra la fecha del instante ISO como AAAA-MM-DD o como DD/MM/AAAA.
function showsDate(text: string, isoInstant: string): boolean {
  const isoDate = isoInstant.slice(0, 10);
  const [year, month, day] = isoDate.split("-");
  return text.includes(isoDate) || text.includes(`${day}/${month}/${year}`);
}

// Hace que fetch responda siempre con el estado y el cuerpo indicados.
function respondWith(
  fetchMock: FetchMock,
  status: number,
  body: StoryDetail | ErrorResponse,
) {
  fetchMock.mockImplementation(
    async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
}

// Extrae la URL, el método y la cabecera Authorization de la última llamada a fetch.
function lastRequest(fetchMock: FetchMock) {
  const [input, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1];
  if (input instanceof Request) {
    return {
      url: input.url,
      method: input.method,
      authorization: input.headers.get("Authorization"),
    };
  }
  return {
    url: input instanceof URL ? input.href : input,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

describe("003/US3 - Consultar el detalle de una historia", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch sustituido.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: el detalle muestra la descripción, la prioridad Media, los 8 Story Points, los tres
  // criterios en el orden en que se cargaron, el estado, el autor, las fechas de creación y de
  // última actualización, y que está lista para planificar (FR-039).
  it("Detalle completo", async () => {
    const text = renderCardText(readyStory);

    expect(text).toContain("Exportar el reporte de velocidad a PDF");
    expect(text).toContain(
      "El equipo necesita compartir la velocidad con la gerencia.",
    );
    expect(text).toContain("Media");
    expect(text).toContain("Pendiente");
    expect(text).toMatch(/\b8 Story Points\b/);
    expect(text).not.toMatch(/sin estimar/i);
    expect(text).toContain("Bruno Gómez");
    expect(showsDate(text, readyStory.createdAt)).toBe(true);
    expect(showsDate(text, readyStory.updatedAt)).toBe(true);
    expect(text).toMatch(/lista para planificar/i);
    expect(text).not.toMatch(/no lista para planificar/i);
    expect(listItems("Qué falta para planificar") ?? []).toHaveLength(0);

    const criteria = listItems("Criterios de aceptación");
    expect(criteria).not.toBeNull();
    expect(criteria).toHaveLength(3);
    criteria?.forEach((item, index) => {
      expect(item).toContain(readyStory.acceptanceCriteria[index]);
    });

    // La llamada de la pantalla entrega el detalle tal cual lo envía el backend.
    respondWith(fetchMock, 200, readyStory);
    const result = await getStory(readyStory.id);
    expect(result).toEqual({ ok: true, data: readyStory });
  });

  // Caso alternativo: sin estimar y sin criterios, el detalle muestra la marca "Sin estimar", la
  // lista de criterios vacía y que no está lista para planificar, con los dos motivos (FR-040).
  it("Sin estimar ni criterios", () => {
    const text = renderCardText(unestimatedStory);

    expect(text).toContain("Notificar por correo el cierre del sprint");
    expect(text).toContain("Baja");
    expect(text).toMatch(/sin estimar/i);
    expect(text).not.toMatch(/\b\d+ Story Points\b/);
    expect(listItems("Criterios de aceptación") ?? []).toHaveLength(0);
    expect(text).toMatch(/no lista para planificar/i);

    const missing = listItems("Qué falta para planificar");
    expect(missing).not.toBeNull();
    expect(missing).toHaveLength(2);
    expect(missing?.some((item) => /estima|story points/i.test(item))).toBe(
      true,
    );
    expect(
      missing?.some((item) => /criterios? de aceptación/i.test(item)),
    ).toBe(true);
    cleanup();

    // Con un solo motivo, la vista indica solo lo que falta.
    renderCardText({
      ...readyStory,
      acceptanceCriteria: [],
      readiness: { ready: false, missing: ["acceptance_criteria"] },
    });
    const onlyCriteria = listItems("Qué falta para planificar");
    expect(onlyCriteria).toHaveLength(1);
    expect(onlyCriteria?.[0]).toMatch(/criterios? de aceptación/i);
  });

  // Caso alternativo: una historia que fue asignada a un sprint muestra, además de sus datos, que
  // está o estuvo asignada a un sprint; una que nunca lo fue no lo menciona.
  it("Asignada a un sprint", () => {
    const assignedText = renderCardText({
      ...readyStory,
      everAssignedToSprint: true,
    });
    expect(assignedText).toContain("Exportar el reporte de velocidad a PDF");
    expect(assignedText).toMatch(/asignada a un sprint/i);
    cleanup();

    const neverAssignedText = renderCardText(readyStory);
    expect(neverAssignedText).not.toMatch(/asignada a un sprint/i);
  });

  // Caso límite: una historia estimada en 0 Story Points muestra "0 Story Points", claramente
  // distinto de "Sin estimar", y con criterios sigue lista para planificar (RN-07).
  it("Cero Story Points", () => {
    const text = renderCardText({ ...readyStory, storyPoints: 0 });

    expect(text).toMatch(/\b0 Story Points\b/);
    expect(text).not.toMatch(/sin estimar/i);
    expect(text).toMatch(/lista para planificar/i);
    expect(text).not.toMatch(/no lista para planificar/i);
  });

  // Caso de error: la consulta de una historia de un proyecto ajeno recibe el 404 de historia
  // inexistente y devuelve solo el error, sin ningún dato de la historia (FR-044, FR-047).
  it("Historia ajena", async () => {
    const message = "La historia no existe.";
    respondWith(fetchMock, 404, { error: message });

    const result = await getStory(readyStory.id);

    expect(result).toEqual({
      ok: false,
      error: { status: 404, message, fields: [] },
    });
    expect(JSON.stringify(result)).not.toContain(readyStory.title);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = lastRequest(fetchMock);
    expect(request.url.endsWith(`/api/v1/stories/${readyStory.id}`)).toBe(true);
    expect(request.method).toBe("GET");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  });

  // Caso de error: un identificador inexistente recibe la misma respuesta que la historia ajena,
  // así que el resultado de las dos consultas es idéntico (SC-006).
  it("Historia inexistente", async () => {
    const message = "La historia no existe.";
    respondWith(fetchMock, 404, { error: message });
    const unknownID = "00000000-0000-4000-8000-000000000000";

    const foreign = await getStory(readyStory.id);
    const unknown = await getStory(unknownID);

    expect(unknown).toEqual({
      ok: false,
      error: { status: 404, message, fields: [] },
    });
    expect(unknown).toEqual(foreign);
    expect(
      lastRequest(fetchMock).url.endsWith(`/api/v1/stories/${unknownID}`),
    ).toBe(true);
  });
});
