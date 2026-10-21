import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { EstimateControl } from "@/features/backlog/components/estimate-control";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type { StoryDetail, StoryPoints } from "@/types/story";

// Pruebas del control de estimación de una historia (T050). Atraviesan el componente, la llamada de
// estimación de api.ts (T055) y el cliente HTTP real; solo se sustituye fetch, que es la frontera con
// el backend. Las reglas (escala, historia Completada, comprometida, proyecto Finalizado) las
// resuelve el backend: acá se verifica qué envía el control y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas:
// - EstimateControl recibe storyId, storyPoints (el valor vigente, null si está sin estimar) y
//   onEstimated, que recibe el StoryDetail devuelto por el backend para que la pantalla de detalle
//   lo muestre.
// - El valor se elige en un selector con la etiqueta "Story Points", que ofrece solo "Sin estimar"
//   (valor "") y los valores de la escala, cada uno con el número como texto y como valor, y arranca
//   con el valor vigente elegido. No hay campo de texto ni numérico libre.
// - El botón "Guardar estimación" envía PUT /api/v1/stories/{storyId}/story-points con la sesión de
//   la pestaña y el cuerpo {"storyPoints": <valor>} o {"storyPoints": null} para "Sin estimar".
// - Con un 200 llama una vez a onEstimated con el StoryDetail recibido.
// - Con un rechazo muestra el motivo con role="alert" (en un 400, el mensaje por campo de
//   storyPoints, que indica la escala) y no llama a onEstimated: el valor vigente no cambia.

type FetchMock = Mock<typeof fetch>;
type EstimatedMock = Mock<(story: StoryDetail) => void>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";

// Mensajes de contracts/openapi.yaml para los rechazos de la estimación.
const scaleMessage =
  "Los valores admitidos de Story Points son 0, 1, 2, 3, 5, 8, 13, 20, 40 y 100, o sin estimar.";
const committedMessage =
  "La historia está comprometida en un sprint abierto; primero hay que quitarla del sprint.";
const completedMessage =
  "La historia está completada y no se puede modificar ni reestimar.";
const readOnlyMessage =
  "El proyecto está finalizado y su backlog es de solo lectura.";

// Textos de las opciones del selector, en el orden de la escala (FR-014).
const scaleOptionTexts = [
  "Sin estimar",
  "0",
  "1",
  "2",
  "3",
  "5",
  "8",
  "13",
  "20",
  "40",
  "100",
];

// Historia sin estimar con dos criterios de aceptación (US4-1).
const unestimatedStory: StoryDetail = {
  id: "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f",
  projectId: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
  title: "Exportar el reporte de velocidad a PDF",
  description: null,
  priority: "high",
  status: "pending",
  storyPoints: null,
  acceptanceCriteria: [
    "El PDF incluye la velocidad de los últimos cinco sprints.",
    "Al pie figura la fecha de generación.",
  ],
  readiness: { ready: false, missing: ["story_points"] },
  createdBy: {
    id: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
    fullName: "Ana Pérez",
  },
  createdAt: "2026-10-10T14:30:00Z",
  updatedAt: "2026-10-10T14:30:00Z",
  everAssignedToSprint: false,
};

// Arma la respuesta 200 del backend: la historia con el valor nuevo, la condición de lista para
// planificar que corresponde y un momento de última actualización posterior.
function estimated(
  story: StoryDetail,
  storyPoints: StoryPoints,
  updatedAt = "2026-10-15T09:00:00Z",
): StoryDetail {
  const missing: StoryDetail["readiness"]["missing"] = [];
  if (storyPoints === null) {
    missing.push("story_points");
  }
  if (story.acceptanceCriteria.length === 0) {
    missing.push("acceptance_criteria");
  }
  return {
    ...story,
    storyPoints,
    readiness: { ready: missing.length === 0, missing },
    updatedAt,
  };
}

// Hace que fetch responda una vez con el estado y el cuerpo indicados.
function respondOnce(
  fetchMock: FetchMock,
  status: number,
  body: StoryDetail | ErrorResponse | ValidationErrorResponse,
) {
  fetchMock.mockImplementationOnce(
    async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
}

// Extrae la URL, el método, la cabecera Authorization y el cuerpo de la última llamada a fetch.
function lastRequest(fetchMock: FetchMock) {
  const [input, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1];
  const url =
    input instanceof Request
      ? input.url
      : input instanceof URL
        ? input.href
        : input;
  return {
    url,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
    body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
  };
}

// Monta el control con el valor vigente indicado y devuelve el doble de onEstimated.
function renderControl(story: StoryDetail): EstimatedMock {
  const onEstimated: EstimatedMock = vi.fn();
  render(
    <EstimateControl
      storyId={story.id}
      storyPoints={story.storyPoints}
      onEstimated={onEstimated}
    />,
  );
  return onEstimated;
}

// Devuelve el selector de Story Points del control.
function pointsSelect(): HTMLSelectElement {
  const select = screen.getByLabelText("Story Points");
  if (!(select instanceof HTMLSelectElement)) {
    throw new Error("Story Points debe ser un selector");
  }
  return select;
}

// Elige un valor del selector y guarda la estimación: los dos pasos del flujo de estimación. El
// valor "" es "Sin estimar".
function estimate(value: string) {
  fireEvent.change(pointsSelect(), { target: { value } });
  fireEvent.click(screen.getByRole("button", { name: "Guardar estimación" }));
}

// Verifica que se envió el PUT de la estimación de la historia con la sesión y el cuerpo indicado.
function expectEstimateRequest(
  fetchMock: FetchMock,
  storyId: string,
  storyPoints: number | null,
) {
  const request = lastRequest(fetchMock);
  expect(request.url.endsWith(`/api/v1/stories/${storyId}/story-points`)).toBe(
    true,
  );
  expect(request.method).toBe("PUT");
  expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  expect(request.body).toEqual({ storyPoints });
}

describe("003/US4 - Estimar una historia en Story Points", () => {
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

  // Caso normal: una historia sin estimar con dos criterios se estima en 13; el control envía el
  // valor y entrega a la pantalla la historia estimada y lista para planificar (FR-016).
  it("Estimación inicial", async () => {
    const response = estimated(unestimatedStory, 13);
    respondOnce(fetchMock, 200, response);
    const onEstimated = renderControl(unestimatedStory);
    expect(pointsSelect().value).toBe("");

    estimate("13");

    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(1));
    expect(onEstimated).toHaveBeenCalledWith(response);
    expect(onEstimated.mock.calls[0][0].readiness.ready).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expectEstimateRequest(fetchMock, unestimatedStory.id, 13);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: una historia estimada en 5 arranca con 5 elegido y se reestima en 8; el valor
  // nuevo reemplaza al anterior con un momento de actualización posterior (FR-017).
  it("Reestimación", async () => {
    const story = estimated(unestimatedStory, 5, "2026-10-12T10:00:00Z");
    const response = estimated(story, 8, "2026-10-16T11:30:00Z");
    respondOnce(fetchMock, 200, response);
    const onEstimated = renderControl(story);
    expect(pointsSelect().value).toBe("5");

    estimate("8");

    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(1));
    expect(onEstimated).toHaveBeenCalledWith(response);
    expectEstimateRequest(fetchMock, story.id, 8);
  });

  // Caso alternativo: una historia estimada no comprometida vuelve a "Sin estimar"; el control
  // envía null y la historia recibida deja de estar lista para planificar (FR-018).
  it("Volver a sin estimar", async () => {
    const story = estimated(unestimatedStory, 8);
    const response = estimated(story, null, "2026-10-16T12:00:00Z");
    respondOnce(fetchMock, 200, response);
    const onEstimated = renderControl(story);
    expect(pointsSelect().value).toBe("8");

    estimate("");

    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(1));
    expect(onEstimated).toHaveBeenCalledWith(response);
    expect(onEstimated.mock.calls[0][0].readiness.ready).toBe(false);
    expectEstimateRequest(fetchMock, story.id, null);
  });

  // Caso límite: una historia sin criterios se estima en 3; la estimación se registra, pero la
  // historia recibida sigue sin estar lista porque le faltan los criterios (RN-12).
  it("Sin criterios", async () => {
    const story: StoryDetail = {
      ...unestimatedStory,
      acceptanceCriteria: [],
      readiness: {
        ready: false,
        missing: ["story_points", "acceptance_criteria"],
      },
    };
    const response = estimated(story, 3);
    respondOnce(fetchMock, 200, response);
    const onEstimated = renderControl(story);

    estimate("3");

    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(1));
    expect(onEstimated).toHaveBeenCalledWith(response);
    expect(onEstimated.mock.calls[0][0].readiness).toEqual({
      ready: false,
      missing: ["acceptance_criteria"],
    });
    expectEstimateRequest(fetchMock, story.id, 3);
  });

  // Caso límite: los extremos de la escala, 0 y 100, se ofrecen y se envían; el 0 viaja como 0 y no
  // como "sin estimar" (FR-015).
  it("Extremos de la escala", async () => {
    const zero = estimated(unestimatedStory, 0);
    const hundred = estimated(zero, 100, "2026-10-16T13:00:00Z");
    respondOnce(fetchMock, 200, zero);
    respondOnce(fetchMock, 200, hundred);
    const onEstimated = renderControl(unestimatedStory);

    estimate("0");
    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(1));
    expect(onEstimated).toHaveBeenLastCalledWith(zero);
    expectEstimateRequest(fetchMock, unestimatedStory.id, 0);

    estimate("100");
    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(2));
    expect(onEstimated).toHaveBeenLastCalledWith(hundred);
    expectEstimateRequest(fetchMock, unestimatedStory.id, 100);
  });

  // Caso de error: una historia comprometida en un sprint abierto no vuelve a "Sin estimar"; el
  // control muestra que primero hay que quitarla del sprint y la estimación no cambia (RN-18).
  it("Comprometida no vuelve a sin estimar", async () => {
    const story = estimated(unestimatedStory, 8);
    respondOnce(fetchMock, 409, { error: committedMessage });
    const onEstimated = renderControl(story);

    estimate("");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(committedMessage);
    expectEstimateRequest(fetchMock, story.id, null);
    expect(onEstimated).not.toHaveBeenCalled();
  });

  // Caso de error: 50, 4 y 6,5 no se pueden elegir porque el selector solo ofrece la escala; si el
  // backend rechaza el valor con 400, el control muestra la escala admitida y la estimación no
  // cambia (FR-019).
  it("Valor fuera de escala", async () => {
    const story = estimated(unestimatedStory, 5);
    respondOnce(fetchMock, 400, {
      error: "Hay campos inválidos en la solicitud.",
      fields: [{ field: "storyPoints", message: scaleMessage }],
    });
    const onEstimated = renderControl(story);

    const offered = Array.from(pointsSelect().options).map(
      (option) => option.value,
    );
    ["50", "4", "6.5", "6,5"].forEach((value) => {
      expect(offered).not.toContain(value);
    });

    estimate("13");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("0, 1, 2, 3, 5, 8, 13, 20, 40 y 100");
    expect(onEstimated).not.toHaveBeenCalled();
  });

  // Caso de error: una historia Completada no se reestima; el control muestra el motivo del rechazo
  // y la estimación no cambia (FR-030, RN-14).
  it("Historia completada", async () => {
    const story: StoryDetail = {
      ...estimated(unestimatedStory, 5),
      status: "completed",
    };
    respondOnce(fetchMock, 409, { error: completedMessage });
    const onEstimated = renderControl(story);

    estimate("8");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(completedMessage);
    expectEstimateRequest(fetchMock, story.id, 8);
    expect(onEstimated).not.toHaveBeenCalled();
  });

  // Caso de error: en un proyecto Finalizado el backlog es de solo lectura; el control muestra el
  // motivo y la historia sigue sin estimar (FR-048, RN-15).
  it("Proyecto finalizado", async () => {
    respondOnce(fetchMock, 409, { error: readOnlyMessage });
    const onEstimated = renderControl(unestimatedStory);

    estimate("5");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(readOnlyMessage);
    expect(onEstimated).not.toHaveBeenCalled();
  });

  // El control ofrece solo los valores de la escala más "Sin estimar", en ese orden, y no admite
  // valores libres: no hay campo de texto ni numérico (FR-014).
  it("Ofrece solo la escala y Sin estimar", () => {
    renderControl(unestimatedStory);

    const options = Array.from(pointsSelect().options);
    expect(options.map((option) => option.textContent)).toEqual(
      scaleOptionTexts,
    );
    expect(options.map((option) => option.value)).toEqual([
      "",
      ...scaleOptionTexts.slice(1),
    ]);
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("spinbutton")).toBeNull();
  });

  // SC-002, parte automatizable: desde el detalle, la estimación se completa en no más de 3 pasos
  // (elegir el valor y guardar), contando cada interacción del integrante con el control.
  it("Estima en no más de 3 pasos (SC-002)", async () => {
    const response = estimated(unestimatedStory, 8);
    respondOnce(fetchMock, 200, response);
    const onEstimated = renderControl(unestimatedStory);
    let steps = 0;

    fireEvent.change(pointsSelect(), { target: { value: "8" } });
    steps += 1;
    fireEvent.click(screen.getByRole("button", { name: "Guardar estimación" }));
    steps += 1;

    await waitFor(() => expect(onEstimated).toHaveBeenCalledTimes(1));
    expect(onEstimated).toHaveBeenCalledWith(response);
    expect(steps).toBeLessThanOrEqual(3);
    expectEstimateRequest(fetchMock, unestimatedStory.id, 8);
  });
});
