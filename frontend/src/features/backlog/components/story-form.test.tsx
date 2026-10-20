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
import { StoryForm } from "@/features/backlog/components/story-form";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";

// Pruebas del formulario de registro de una historia (T016). Atraviesan el componente, la
// validación del formulario y el cliente HTTP real; solo se sustituye fetch, que es la frontera con
// el backend.
//
// Contrato de accesibilidad que fijan estas pruebas para el formulario, igual que en la 001 y la 002:
// - Cada campo se encuentra por su etiqueta: "Título", "Descripción", "Prioridad" (selector con
//   Alta, Media y Baja, sin valor elegido al inicio) y "Story Points" (selector con "Sin estimar"
//   elegido al inicio y los valores de la escala).
// - Los criterios de aceptación se agregan con el botón "Agregar criterio"; cada uno es un campo de
//   texto llamado "Criterio N" (o "Criterio de aceptación N"), numerado desde 1 en el orden de la
//   lista.
// - Las indicaciones de los límites y los errores de cada campo se asocian al campo con
//   aria-describedby; un campo con error lleva aria-invalid="true".
// - Los errores generales (proyecto finalizado, proyecto inexistente) se muestran con role="alert".
// - Al crearse la historia, el formulario confirma "Historia creada" con su título, el estado
//   Pendiente, "Sin estimar" si no tiene Story Points y "Lista para planificar" o "No lista para
//   planificar" según la respuesta del backend.

// Sustituye el enrutador de Next.js por si el formulario lo usa: fuera del App Router no hay
// enrutador montado.
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    refresh: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    prefetch: vi.fn(),
  }),
}));

type FetchMock = Mock<typeof fetch>;

// Cuerpo de POST /api/v1/projects/{projectId}/stories según StoryInput de contracts/openapi.yaml.
// Se declara acá para leer lo que envía el formulario sin depender de los tipos de T025.
type SentStoryInput = {
  title: string;
  description?: string | null;
  priority: string;
  storyPoints?: number | null;
  acceptanceCriteria?: string[];
};

// Respuesta 201 según StoryDetail de contracts/openapi.yaml.
type CreatedStory = {
  id: string;
  projectId: string;
  title: string;
  description: string | null;
  priority: "high" | "medium" | "low";
  status: "pending" | "in_progress" | "completed";
  storyPoints: number | null;
  acceptanceCriteria: string[];
  readiness: { ready: boolean; missing: string[] };
  createdBy: { id: string; fullName: string };
  createdAt: string;
  updatedAt: string;
  everAssignedToSprint: boolean;
};

// Valores tal como se eligen en el formulario: prioridad y Story Points son el valor del selector
// ("" es "sin elegir" en la prioridad); los criterios se escriben en el orden de la lista.
type FormValues = {
  title: string;
  description: string;
  priority: string;
  storyPoints: string;
  criteria: string[];
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const storiesPath = `/api/v1/projects/${projectId}/stories`;

const firstCriterion =
  "El PDF incluye la velocidad de los últimos cinco sprints.";
const secondCriterion = "El archivo se descarga con el nombre del proyecto.";

// Escala de Story Points admitida (FR-014), tal como la enumera la spec.
const storyPointsScale = [
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

const completeValues: FormValues = {
  title: "Exportar el reporte de velocidad a PDF",
  description: "El equipo necesita compartir la velocidad con la gerencia.",
  priority: "high",
  storyPoints: "5",
  criteria: [firstCriterion, secondCriterion],
};

const minimalValues: FormValues = {
  title: "Revisar los permisos de los integrantes",
  description: "",
  priority: "low",
  storyPoints: "",
  criteria: [],
};

const createdStory: CreatedStory = {
  id: "5e0b7c1d-2a3f-4b6c-8d9e-0f1a2b3c4d5e",
  projectId,
  title: "Exportar el reporte de velocidad a PDF",
  description: "El equipo necesita compartir la velocidad con la gerencia.",
  priority: "high",
  status: "pending",
  storyPoints: 5,
  acceptanceCriteria: [firstCriterion, secondCriterion],
  readiness: { ready: true, missing: [] },
  createdBy: {
    id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
    fullName: "Ana Pérez",
  },
  createdAt: "2026-10-20T12:00:00Z",
  updatedAt: "2026-10-20T12:00:00Z",
  everAssignedToSprint: false,
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body: CreatedStory | ErrorResponse | ValidationErrorResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Responde 201 con la historia que el backend armaría a partir del cuerpo recibido: Pendiente, con
// los Story Points y los criterios enviados y la condición de lista para planificar de RN-12.
function respondCreated() {
  return async (_input: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body)) as SentStoryInput;
    const storyPoints = body.storyPoints ?? null;
    const criteria = (body.acceptanceCriteria ?? []).map((c) => c.trim());
    const missing: string[] = [];
    if (storyPoints === null) {
      missing.push("story_points");
    }
    if (criteria.length === 0) {
      missing.push("acceptance_criteria");
    }
    const description = (body.description ?? "").trim();
    return jsonResponse(201, {
      ...createdStory,
      title: body.title.trim(),
      description: description === "" ? null : description,
      priority: body.priority as CreatedStory["priority"],
      storyPoints,
      acceptanceCriteria: criteria,
      readiness: { ready: missing.length === 0, missing },
    });
  };
}

// Devuelve el texto de todos los elementos que describen a un campo vía aria-describedby.
function describedText(element: HTMLElement): string {
  const ids = (element.getAttribute("aria-describedby") ?? "")
    .split(/\s+/)
    .filter(Boolean);
  return ids
    .map((id) => document.getElementById(id)?.textContent ?? "")
    .join(" ");
}

// Localiza los campos fijos y el botón de envío del formulario.
function getFields() {
  return {
    title: screen.getByLabelText<HTMLInputElement>(/^título/i),
    description: screen.getByLabelText<HTMLTextAreaElement>(/^descripción/i),
    priority: screen.getByLabelText<HTMLSelectElement>(/^prioridad/i),
    storyPoints: screen.getByLabelText<HTMLSelectElement>(/^story points/i),
    submit: screen.getByRole("button", { name: /crear historia/i }),
  };
}

// Localiza los campos de los criterios de aceptación en el orden de la lista.
function getCriterionFields(): HTMLElement[] {
  return screen.queryAllByRole("textbox", {
    name: /^criterio( de aceptación)? \d+/i,
  });
}

// Agrega un criterio con el botón del formulario y devuelve su campo, que es el último de la lista.
function addCriterion(): HTMLElement {
  fireEvent.click(screen.getByRole("button", { name: /agregar criterio/i }));
  const criteria = getCriterionFields();
  return criteria[criteria.length - 1];
}

// Completa el formulario con los valores dados, agregando un criterio por cada texto, y lo envía.
function fillAndSubmit(values: FormValues) {
  const fields = getFields();
  fireEvent.change(fields.title, { target: { value: values.title } });
  fireEvent.change(fields.description, {
    target: { value: values.description },
  });
  fireEvent.change(fields.priority, { target: { value: values.priority } });
  if (values.storyPoints !== "") {
    fireEvent.change(fields.storyPoints, {
      target: { value: values.storyPoints },
    });
  }
  for (const text of values.criteria) {
    fireEvent.change(addCriterion(), { target: { value: text } });
  }
  fireEvent.click(fields.submit);
  return { ...fields, criteria: getCriterionFields() };
}

// Extrae la URL, el método, la cabecera Authorization y el cuerpo JSON de la llamada número index
// a fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const authorization = new Headers(init?.headers).get("Authorization");
  const body = JSON.parse(String(init?.body)) as SentStoryInput;
  return { url, method: init?.method, authorization, body };
}

// Espera a que el campo dado quede marcado como inválido, sin que se haya llamado al backend.
async function expectRejectedBeforeSending(
  fetchMock: FetchMock,
  field: HTMLElement,
) {
  await waitFor(() => expect(field.getAttribute("aria-invalid")).toBe("true"));
  expect(fetchMock).not.toHaveBeenCalled();
  expect(screen.queryByText(/historia creada/i)).toBeNull();
}

// Texto de la página sin las opciones de los selectores, para que "Sin estimar" del selector de
// Story Points no se confunda con la confirmación de la historia creada.
function pageTextWithoutSelectors(): string {
  const copy = document.body.cloneNode(true) as HTMLElement;
  copy.querySelectorAll("select").forEach((select) => select.remove());
  return copy.textContent ?? "";
}

// Texto visible de las opciones de un selector.
function optionTexts(select: HTMLSelectElement): string[] {
  return Array.from(select.options).map((o) => (o.textContent ?? "").trim());
}

describe("003/US1 - Registrar una historia en el backlog", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que responde 201.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(respondCreated());
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Los límites del título, de la descripción y de cada criterio se comunican junto a cada campo
  // desde el inicio, sin haber enviado nada y sin que ningún campo esté marcado como inválido.
  it("muestra antes de enviar los 150 caracteres del título, los 2000 de la descripción y los 500 de cada criterio", () => {
    render(<StoryForm projectId={projectId} />);
    const { title, description } = getFields();
    const criterion = addCriterion();

    expect(describedText(title)).toMatch(/150/);
    expect(describedText(title)).toMatch(/caracteres/i);
    expect(describedText(description)).toMatch(/2000|2\.000/);
    expect(describedText(description)).toMatch(/caracteres/i);
    expect(describedText(criterion)).toMatch(/500/);
    expect(describedText(criterion)).toMatch(/caracteres/i);

    for (const field of [title, description, criterion]) {
      expect(field.getAttribute("aria-invalid")).not.toBe("true");
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // FR-014 y FR-005: el selector de Story Points ofrece solo los valores de la escala más "sin
  // estimar", que es la opción elegida al inicio; el de prioridad ofrece Alta, Media y Baja y empieza
  // sin valor elegido.
  it("ofrece en el selector de Story Points solo la escala y sin estimar", () => {
    render(<StoryForm projectId={projectId} />);
    const { storyPoints, priority } = getFields();

    const options = Array.from(storyPoints.options);
    expect(options).toHaveLength(storyPointsScale.length + 1);
    const unestimated = options.filter((o) =>
      /sin estimar/i.test(o.textContent ?? ""),
    );
    expect(unestimated).toHaveLength(1);
    expect(storyPoints.selectedOptions[0]).toBe(unestimated[0]);
    expect(
      options.filter((o) => o !== unestimated[0]).map((o) => o.value),
    ).toEqual(storyPointsScale);

    const priorityValues = Array.from(priority.options)
      .map((o) => o.value)
      .filter((v) => v !== "");
    expect(priorityValues).toEqual(["high", "medium", "low"]);
    const texts = optionTexts(priority);
    for (const label of ["Alta", "Media", "Baja"]) {
      expect(texts).toContain(label);
    }
    expect(priority.value).toBe("");
  });

  // Caso normal: el formulario envía POST /api/v1/projects/{projectId}/stories con la sesión de la
  // pestaña, los dos criterios en orden y 5 Story Points como número, y confirma la historia
  // Pendiente y lista para planificar.
  it("Creación completa", async () => {
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit(completeValues);

    expect(await screen.findByText(/historia creada/i)).toBeTruthy();
    const text = pageTextWithoutSelectors();
    expect(text).toContain("Exportar el reporte de velocidad a PDF");
    expect(text).toContain("Pendiente");
    expect(text).toMatch(/lista para planificar/i);
    expect(text).not.toMatch(/no lista para planificar/i);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(storiesPath)).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toEqual({
      title: "Exportar el reporte de velocidad a PDF",
      description: "El equipo necesita compartir la velocidad con la gerencia.",
      priority: "high",
      storyPoints: 5,
      acceptanceCriteria: [firstCriterion, secondCriterion],
    });
  });

  // Caso alternativo: con solo título y prioridad la historia se envía sin Story Points (nunca 0),
  // sin descripción y sin criterios, y se confirma "sin estimar" y no lista para planificar.
  it("Solo título y prioridad", async () => {
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit(minimalValues);

    expect(await screen.findByText(/historia creada/i)).toBeTruthy();
    const text = pageTextWithoutSelectors();
    expect(text).toContain("Revisar los permisos de los integrantes");
    expect(text).toContain("Pendiente");
    expect(text).toMatch(/sin estimar/i);
    expect(text).toMatch(/no lista para planificar/i);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const { body } = requestAt(fetchMock, 0);
    expect(body.title).toBe("Revisar los permisos de los integrantes");
    expect(body.priority).toBe("low");
    expect(body.storyPoints ?? null).toBeNull();
    expect(body.description ?? "").toBe("");
    expect(body.acceptanceCriteria ?? []).toEqual([]);
  });

  // Caso alternativo: el formulario no bloquea la creación por el estado del proyecto; en un
  // proyecto Planificado el backend acepta y el formulario confirma la historia Pendiente.
  it("Proyecto planificado", async () => {
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit({ ...minimalValues, priority: "medium" });

    expect(await screen.findByText(/historia creada/i)).toBeTruthy();
    expect(document.body.textContent).toContain("Pendiente");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestAt(fetchMock, 0).body.priority).toBe("medium");
  });

  // Caso límite: 0 Story Points se envía como el número 0, distinto de "sin estimar", y con un
  // criterio la historia queda lista para planificar.
  it("Cero Story Points", async () => {
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit({
      ...completeValues,
      storyPoints: "0",
      criteria: [firstCriterion],
    });

    expect(await screen.findByText(/historia creada/i)).toBeTruthy();
    const text = pageTextWithoutSelectors();
    expect(text).not.toMatch(/sin estimar/i);
    expect(text).not.toMatch(/no lista para planificar/i);
    expect(text).toMatch(/lista para planificar/i);

    const { body } = requestAt(fetchMock, 0);
    expect(body).toHaveProperty("storyPoints", 0);
    expect(body.acceptanceCriteria).toEqual([firstCriterion]);
  });

  // Caso límite: un título de 150 caracteres se envía y uno de 151 se rechaza antes de enviar
  // indicando el máximo.
  it("Límites del título", async () => {
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit({ ...minimalValues, title: "a".repeat(150) });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(requestAt(fetchMock, 0).body.title).toBe("a".repeat(150));
    cleanup();
    fetchMock.mockClear();

    render(<StoryForm projectId={projectId} />);
    const { title } = fillAndSubmit({
      ...minimalValues,
      title: "a".repeat(151),
    });
    await expectRejectedBeforeSending(fetchMock, title);
    expect(describedText(title)).toMatch(/150/);
  });

  // FR-003: una descripción de 2001 caracteres se rechaza antes de enviar indicando el máximo.
  it("Límites de la descripción", async () => {
    render(<StoryForm projectId={projectId} />);
    const { description } = fillAndSubmit({
      ...minimalValues,
      description: "d".repeat(2001),
    });
    await expectRejectedBeforeSending(fetchMock, description);
    expect(describedText(description)).toMatch(/2000|2\.000/);
  });

  // Caso límite: un criterio de exactamente 500 caracteres y otro de 1 carácter se envían, en ese
  // orden; uno de 501 se rechaza antes de enviar indicando el máximo.
  it("Límites del criterio", async () => {
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit({ ...completeValues, criteria: ["c".repeat(500), "x"] });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(requestAt(fetchMock, 0).body.acceptanceCriteria).toEqual([
      "c".repeat(500),
      "x",
    ]);
    cleanup();
    fetchMock.mockClear();

    render(<StoryForm projectId={projectId} />);
    const { criteria } = fillAndSubmit({
      ...completeValues,
      criteria: [firstCriterion, "c".repeat(501)],
    });
    await expectRejectedBeforeSending(fetchMock, criteria[1]);
    expect(describedText(criteria[1])).toMatch(/500/);
    expect(criteria[0].getAttribute("aria-invalid")).not.toBe("true");
  });

  // Caso límite: el formulario no bloquea un título ya usado; dos historias con el mismo título se
  // envían y ambas se confirman.
  it("Título repetido", async () => {
    for (let index = 0; index < 2; index++) {
      render(<StoryForm projectId={projectId} />);
      fillAndSubmit(completeValues);
      expect(await screen.findByText(/historia creada/i)).toBeTruthy();
      cleanup();
    }
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(requestAt(fetchMock, 0).body.title).toBe(completeValues.title);
    expect(requestAt(fetchMock, 1).body.title).toBe(completeValues.title);
  });

  // Caso de error: sin título, con título solo de espacios o sin prioridad no se envía nada y cada
  // campo indica qué corregir; un 400 del backend se ubica junto al campo que informa.
  it("Campos obligatorios", async () => {
    const cases: Array<
      [string, Partial<FormValues>, "title" | "priority", RegExp]
    > = [
      ["sin título", { title: "" }, "title", /título/i],
      ["título solo de espacios", { title: "   \t " }, "title", /título/i],
      ["sin prioridad", { priority: "" }, "priority", /prioridad/i],
    ];
    for (const [label, changes, field, motive] of cases) {
      render(<StoryForm projectId={projectId} />);
      const fields = fillAndSubmit({ ...completeValues, ...changes });
      await waitFor(() =>
        expect(fields[field].getAttribute("aria-invalid"), label).toBe("true"),
      );
      expect(describedText(fields[field]), label).toMatch(motive);
      expect(fetchMock, label).not.toHaveBeenCalled();
      expect(screen.queryByText(/historia creada/i), label).toBeNull();
      cleanup();
    }

    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [{ field: "title", message: "El título es obligatorio." }],
      }),
    );
    render(<StoryForm projectId={projectId} />);
    const { title } = fillAndSubmit(completeValues);
    await waitFor(() =>
      expect(title.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(title)).toContain("El título es obligatorio.");
    expect(screen.queryByText(/historia creada/i)).toBeNull();
  });

  // Caso de error: el selector no permite elegir 4, 7, un negativo ni un decimal; si el backend
  // rechaza los Story Points con 400, el mensaje con la escala aparece junto al selector.
  it("Story Points fuera de escala", async () => {
    render(<StoryForm projectId={projectId} />);
    const values = Array.from(getFields().storyPoints.options).map(
      (o) => o.value,
    );
    for (const invalid of ["4", "7", "-1", "6.5"]) {
      expect(values, `valor ${invalid}`).not.toContain(invalid);
    }
    cleanup();

    const scaleMessage =
      "Los Story Points deben ser uno de 0, 1, 2, 3, 5, 8, 13, 20, 40 o 100.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [{ field: "storyPoints", message: scaleMessage }],
      }),
    );
    render(<StoryForm projectId={projectId} />);
    const { storyPoints } = fillAndSubmit(completeValues);
    await waitFor(() =>
      expect(storyPoints.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(storyPoints)).toContain(scaleMessage);
    expect(screen.queryByText(/historia creada/i)).toBeNull();
  });

  // Caso de error: un criterio vacío o solo de espacios se rechaza antes de enviar indicando que
  // cada criterio debe tener contenido; un 400 del backend sobre acceptanceCriteria[1] se ubica
  // junto al segundo criterio.
  it("Criterio vacío", async () => {
    for (const empty of ["", "   "]) {
      render(<StoryForm projectId={projectId} />);
      const { criteria } = fillAndSubmit({
        ...completeValues,
        criteria: [firstCriterion, empty],
      });
      await expectRejectedBeforeSending(fetchMock, criteria[1]);
      expect(describedText(criteria[1]), JSON.stringify(empty)).toMatch(
        /contenido/i,
      );
      cleanup();
    }

    const criterionMessage =
      "Cada criterio de aceptación debe tener contenido.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [{ field: "acceptanceCriteria[1]", message: criterionMessage }],
      }),
    );
    render(<StoryForm projectId={projectId} />);
    const { criteria } = fillAndSubmit(completeValues);
    await waitFor(() =>
      expect(criteria[1].getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(criteria[1])).toContain(criterionMessage);
    expect(criteria[0].getAttribute("aria-invalid")).not.toBe("true");
    expect(screen.queryByText(/historia creada/i)).toBeNull();
  });

  // Caso de error: en un proyecto Finalizado el backend responde 409 y el formulario muestra el
  // motivo sin confirmar la creación.
  it("Proyecto finalizado", async () => {
    const message =
      "El proyecto está finalizado y su backlog es de solo lectura.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: message }),
    );
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit(completeValues);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(requestAt(fetchMock, 0).method).toBe("POST");
    expect(screen.queryByText(/historia creada/i)).toBeNull();
  });

  // Caso de error: quien no es integrante recibe 404 de proyecto inexistente y el formulario lo
  // muestra sin confirmar la creación.
  it("No integrante", async () => {
    const message = "El proyecto no existe.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(404, { error: message }),
    );
    render(<StoryForm projectId={projectId} />);
    fillAndSubmit(completeValues);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(screen.queryByText(/historia creada/i)).toBeNull();
  });
});
