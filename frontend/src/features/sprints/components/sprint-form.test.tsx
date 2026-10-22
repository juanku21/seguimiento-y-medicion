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
import { SprintForm } from "@/features/sprints/components/sprint-form";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";

// Pruebas del formulario de creación de un sprint (T014). Atraviesan el componente, la validación
// del formulario y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el backend.
//
// Contrato de accesibilidad que fijan estas pruebas para el formulario, igual que en la 001 a la 003:
// - Cada campo se encuentra por su etiqueta: "Nombre", "Sprint Goal", "Fecha de inicio" y "Fecha de
//   fin". Las fechas son campos de tipo fecha cuyo valor es AAAA-MM-DD.
// - El envío se hace con el botón "Crear sprint".
// - Las indicaciones de los límites y los errores de cada campo se asocian al campo con
//   aria-describedby; un campo con error lleva aria-invalid="true".
// - Los errores generales (superposición, proyecto finalizado, proyecto inexistente) se muestran con
//   role="alert".
// - Al crearse el sprint, el formulario confirma "Sprint creado" con su nombre, el estado Planificado
//   y el nombre de quien lo creó, tomados de la respuesta del backend.

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

// Cuerpo de POST /api/v1/projects/{projectId}/sprints según SprintInput de contracts/openapi.yaml.
// Se declara acá para leer lo que envía el formulario sin depender de los tipos de T021.
type SentSprintInput = {
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
};

// Respuesta 201 según SprintDetail de contracts/openapi.yaml, para un sprint recién creado.
type CreatedSprint = {
  id: string;
  projectId: string;
  name: string;
  goal: string;
  status: "planned" | "active" | "closed";
  startDate: string;
  endDate: string;
  startedAt: string | null;
  closedAt: string | null;
  createdBy: { id: string; fullName: string };
  closedBy: { id: string; fullName: string } | null;
  createdAt: string;
  updatedAt: string;
  stories: never[];
  results: null;
  hoursPerStoryPoint: number | null;
  schedule: null;
};

// Rechazo por conflicto con otro sprint del proyecto, según SprintConflictResponse.
type SprintConflictResponse = {
  error: string;
  sprint: { id: string; name: string };
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintsPath = `/api/v1/projects/${projectId}/sprints`;
const creatorName = "Ana Pérez";

// Datos del caso normal de la spec (US1, escenario 1).
const normalValues: SentSprintInput = {
  name: "Sprint 1",
  goal: "Cerrar el alta de clientes",
  startDate: "2026-10-01",
  endDate: "2026-10-14",
};

const createdSprint: CreatedSprint = {
  id: "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
  projectId,
  name: normalValues.name,
  goal: normalValues.goal,
  status: "planned",
  startDate: normalValues.startDate,
  endDate: normalValues.endDate,
  startedAt: null,
  closedAt: null,
  createdBy: {
    id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
    fullName: creatorName,
  },
  closedBy: null,
  createdAt: "2026-10-20T12:00:00Z",
  updatedAt: "2026-10-20T12:00:00Z",
  stories: [],
  results: null,
  hoursPerStoryPoint: null,
  schedule: null,
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body:
    | CreatedSprint
    | ErrorResponse
    | ValidationErrorResponse
    | SprintConflictResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Responde 201 con el sprint que el backend armaría a partir del cuerpo recibido: Planificado, sin
// historias y atribuido al titular de la sesión.
function respondCreated() {
  return async (_input: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body)) as SentSprintInput;
    return jsonResponse(201, {
      ...createdSprint,
      name: body.name.trim(),
      goal: body.goal.trim(),
      startDate: body.startDate,
      endDate: body.endDate,
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

// Localiza los campos y el botón de envío del formulario.
function getFields() {
  return {
    name: screen.getByLabelText<HTMLInputElement>(/^nombre/i),
    goal: screen.getByLabelText<HTMLInputElement | HTMLTextAreaElement>(
      /^sprint goal/i,
    ),
    startDate: screen.getByLabelText<HTMLInputElement>(/^fecha de inicio/i),
    endDate: screen.getByLabelText<HTMLInputElement>(/^fecha de fin/i),
    submit: screen.getByRole("button", { name: /crear sprint/i }),
  };
}

type FieldName = "name" | "goal" | "startDate" | "endDate";

// Completa el formulario con los valores dados y lo envía. Un campo vacío se deja sin tocar, como
// cuando la persona no lo completa.
function fillAndSubmit(values: SentSprintInput) {
  const fields = getFields();
  const order: FieldName[] = ["name", "goal", "startDate", "endDate"];
  for (const field of order) {
    if (values[field] !== "") {
      fireEvent.change(fields[field], { target: { value: values[field] } });
    }
  }
  fireEvent.click(fields.submit);
  return fields;
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
  const body = JSON.parse(String(init?.body)) as SentSprintInput;
  return { url, method: init?.method, authorization, body };
}

// Espera a que el campo dado quede marcado como inválido, sin que se haya llamado al backend.
async function expectRejectedBeforeSending(
  fetchMock: FetchMock,
  field: HTMLElement,
) {
  await waitFor(() => expect(field.getAttribute("aria-invalid")).toBe("true"));
  expect(fetchMock).not.toHaveBeenCalled();
  expect(screen.queryByText(/sprint creado/i)).toBeNull();
}

// Envía el formulario con los valores dados y espera la confirmación de la creación.
async function submitAndExpectCreated(values: SentSprintInput) {
  render(<SprintForm projectId={projectId} />);
  fillAndSubmit(values);
  expect(await screen.findByText(/sprint creado/i)).toBeTruthy();
}

describe("004/US1 - Planificar un sprint", () => {
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

  // Los límites del nombre y del Sprint Goal se comunican junto a cada campo desde el inicio, sin
  // haber enviado nada y sin que ningún campo esté marcado como inválido (FR-002, FR-003).
  it("muestra antes de enviar los 100 caracteres del nombre y los 500 del Sprint Goal", () => {
    render(<SprintForm projectId={projectId} />);
    const { name, goal } = getFields();

    expect(describedText(name)).toMatch(/100/);
    expect(describedText(name)).toMatch(/caracteres/i);
    expect(describedText(goal)).toMatch(/500/);
    expect(describedText(goal)).toMatch(/caracteres/i);

    for (const field of [name, goal]) {
      expect(field.getAttribute("aria-invalid")).not.toBe("true");
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso normal: el formulario envía POST /api/v1/projects/{projectId}/sprints con la sesión de la
  // pestaña y solo nombre, Sprint Goal y fechas (el autor no es un dato de la entrada, FR-060), y
  // confirma el sprint Planificado atribuido a quien lo creó.
  it("Creación en un proyecto En curso", async () => {
    await submitAndExpectCreated(normalValues);

    const text = document.body.textContent ?? "";
    expect(text).toContain("Sprint 1");
    expect(text).toContain("Planificado");
    expect(text).toContain(creatorName);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(sprintsPath)).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toEqual(normalValues);
  });

  // Caso alternativo: el formulario no bloquea la creación por el estado del proyecto; en un
  // proyecto Planificado el backend acepta y el formulario confirma el sprint Planificado.
  it("Proyecto Planificado", async () => {
    await submitAndExpectCreated(normalValues);

    expect(document.body.textContent).toContain("Planificado");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  // Caso alternativo: con un sprint Planificado ya creado, el formulario envía y confirma otro con
  // fechas que no se superponen.
  it("Varios sprints planificados", async () => {
    await submitAndExpectCreated(normalValues);
    cleanup();
    await submitAndExpectCreated({
      name: "Sprint 2",
      goal: "Publicar el reporte de velocidad",
      startDate: "2026-10-15",
      endDate: "2026-10-28",
    });

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(requestAt(fetchMock, 0).body.name).toBe("Sprint 1");
    expect(requestAt(fetchMock, 1).body.name).toBe("Sprint 2");
    expect(document.body.textContent).toContain("Sprint 2");
  });

  // Caso límite: un sprint que empieza el mismo día en que termina otro no se bloquea en el
  // formulario; se envía y se confirma.
  it("Fechas contiguas", async () => {
    await submitAndExpectCreated({
      ...normalValues,
      name: "Sprint 2",
      startDate: "2026-10-14",
      endDate: "2026-10-28",
    });

    const { body } = requestAt(fetchMock, 0);
    expect(body.startDate).toBe("2026-10-14");
    expect(body.endDate).toBe("2026-10-28");
  });

  // Caso límite: un Sprint Goal de exactamente 500 caracteres se envía y uno de 501 se rechaza antes
  // de enviar indicando el máximo.
  it("Límite del Sprint Goal", async () => {
    await submitAndExpectCreated({ ...normalValues, goal: "g".repeat(500) });
    expect(requestAt(fetchMock, 0).body.goal).toBe("g".repeat(500));
    cleanup();
    fetchMock.mockClear();

    render(<SprintForm projectId={projectId} />);
    const { goal } = fillAndSubmit({ ...normalValues, goal: "g".repeat(501) });
    await expectRejectedBeforeSending(fetchMock, goal);
    expect(describedText(goal)).toMatch(/500/);
  });

  // FR-002: un nombre de exactamente 100 caracteres se envía y uno de 101 se rechaza antes de enviar
  // indicando el máximo.
  it("Límite del nombre", async () => {
    await submitAndExpectCreated({ ...normalValues, name: "a".repeat(100) });
    expect(requestAt(fetchMock, 0).body.name).toBe("a".repeat(100));
    cleanup();
    fetchMock.mockClear();

    render(<SprintForm projectId={projectId} />);
    const { name } = fillAndSubmit({ ...normalValues, name: "a".repeat(101) });
    await expectRejectedBeforeSending(fetchMock, name);
    expect(describedText(name)).toMatch(/100/);
  });

  // Caso límite: una fecha de fin igual al día siguiente de la de inicio es la duración mínima; el
  // formulario la envía y confirma el sprint.
  it("Duración mínima", async () => {
    await submitAndExpectCreated({
      ...normalValues,
      startDate: "2026-10-01",
      endDate: "2026-10-02",
    });

    const { body } = requestAt(fetchMock, 0);
    expect(body.startDate).toBe("2026-10-01");
    expect(body.endDate).toBe("2026-10-02");
  });

  // Caso de error: sin nombre, sin Sprint Goal (vacíos o solo espacios), sin fecha de inicio o sin
  // fecha de fin no se envía nada y el campo indica qué corregir; un 400 del backend se ubica junto
  // al campo que informa.
  it("Campos obligatorios", async () => {
    const cases: Array<[string, Partial<SentSprintInput>, FieldName, RegExp]> =
      [
        ["sin nombre", { name: "" }, "name", /nombre/i],
        ["nombre solo de espacios", { name: "   \t " }, "name", /nombre/i],
        ["sin Sprint Goal", { goal: "" }, "goal", /sprint goal/i],
        [
          "Sprint Goal solo de espacios",
          { goal: "   " },
          "goal",
          /sprint goal/i,
        ],
        ["sin fecha de inicio", { startDate: "" }, "startDate", /inicio/i],
        ["sin fecha de fin", { endDate: "" }, "endDate", /fin/i],
      ];
    for (const [label, changes, field, motive] of cases) {
      render(<SprintForm projectId={projectId} />);
      const fields = fillAndSubmit({ ...normalValues, ...changes });
      await waitFor(() =>
        expect(fields[field].getAttribute("aria-invalid"), label).toBe("true"),
      );
      expect(describedText(fields[field]), label).toMatch(motive);
      expect(fetchMock, label).not.toHaveBeenCalled();
      expect(screen.queryByText(/sprint creado/i), label).toBeNull();
      cleanup();
    }

    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [{ field: "goal", message: "El Sprint Goal es obligatorio." }],
      }),
    );
    render(<SprintForm projectId={projectId} />);
    const { goal } = fillAndSubmit(normalValues);
    await waitFor(() => expect(goal.getAttribute("aria-invalid")).toBe("true"));
    expect(describedText(goal)).toContain("El Sprint Goal es obligatorio.");
    expect(screen.queryByText(/sprint creado/i)).toBeNull();
  });

  // Caso de error: una fecha de fin igual o anterior a la de inicio se avisa antes de enviar, junto
  // a la fecha de fin, indicando que debe ser posterior a la de inicio.
  it("Fin no posterior al inicio", async () => {
    const cases: Array<[string, string, string]> = [
      ["fin igual al inicio", "2026-10-14", "2026-10-14"],
      ["fin anterior al inicio", "2026-10-14", "2026-10-01"],
    ];
    for (const [label, startDate, endDate] of cases) {
      render(<SprintForm projectId={projectId} />);
      const fields = fillAndSubmit({ ...normalValues, startDate, endDate });
      await waitFor(() =>
        expect(fields.endDate.getAttribute("aria-invalid"), label).toBe("true"),
      );
      expect(describedText(fields.endDate), label).toMatch(/posterior/i);
      expect(fetchMock, label).not.toHaveBeenCalled();
      expect(screen.queryByText(/sprint creado/i), label).toBeNull();
      cleanup();
    }
  });

  // Caso de error: si el período se superpone con otro sprint del proyecto, el backend responde 409
  // y el formulario indica con qué sprint se superpone, sin confirmar la creación.
  it("Superposición", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, {
        error: 'El período se superpone con el sprint "Sprint 1".',
        sprint: {
          id: "1d2c3b4a-5f6e-4a7b-8c9d-0e1f2a3b4c5d",
          name: "Sprint 1",
        },
      }),
    );
    render(<SprintForm projectId={projectId} />);
    fillAndSubmit({
      ...normalValues,
      name: "Sprint 2",
      startDate: "2026-10-10",
      endDate: "2026-10-20",
    });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Sprint 1");
    expect(requestAt(fetchMock, 0).method).toBe("POST");
    expect(screen.queryByText(/sprint creado/i)).toBeNull();
  });

  // Caso de error: en un proyecto Finalizado el backend responde 409 y el formulario muestra el
  // motivo sin confirmar la creación.
  it("Proyecto finalizado", async () => {
    const message =
      "El proyecto está finalizado y sus sprints son de solo lectura.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: message }),
    );
    render(<SprintForm projectId={projectId} />);
    fillAndSubmit(normalValues);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(screen.queryByText(/sprint creado/i)).toBeNull();
  });

  // Caso de error: quien no es integrante recibe 404 de proyecto inexistente y el formulario lo
  // muestra sin confirmar la creación.
  it("No integrante", async () => {
    const message = "El proyecto no existe.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(404, { error: message }),
    );
    render(<SprintForm projectId={projectId} />);
    fillAndSubmit(normalValues);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(screen.queryByText(/sprint creado/i)).toBeNull();
  });
});
