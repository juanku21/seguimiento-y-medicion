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
import { ProjectForm } from "@/features/projects/components/project-form";
import {
  readToken,
  wasSessionRejected,
  writeToken,
} from "@/lib/session-storage";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type { ProjectDetail, ProjectInput } from "@/types/project";

// Pruebas del formulario de creación de proyecto (T021). Atraviesan el componente, la validación
// del formulario y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el backend.
//
// Contrato de accesibilidad que fijan estas pruebas para el formulario, igual que en la 001:
// - Cada campo se encuentra por su etiqueta: "Nombre", "Descripción", "Fecha de inicio",
//   "Fecha de finalización prevista" y "Horas por Story Point".
// - Las indicaciones de los límites y los errores de cada campo se asocian al campo con
//   aria-describedby; un campo con error lleva aria-invalid="true".
// - Los errores generales (por ejemplo, nombre repetido) se muestran con role="alert".
// - Al crearse el proyecto, el formulario confirma "Proyecto creado" con su estado Planificado.

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

// Valores tal como se escriben en el formulario: el factor es el texto del campo.
type FormValues = {
  name: string;
  description: string;
  startDate: string;
  plannedEndDate: string;
  hoursPerStoryPoint: string;
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";

const validValues: FormValues = {
  name: "Portal de Clientes",
  description: "Rediseño del portal de autogestión.",
  startDate: "2026-10-01",
  plannedEndDate: "2026-12-20",
  hoursPerStoryPoint: "6.5",
};

const createdProject: ProjectDetail = {
  id: "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e",
  name: "Portal de Clientes",
  description: "Rediseño del portal de autogestión.",
  status: "planned",
  startDate: "2026-10-01",
  plannedEndDate: "2026-12-20",
  actualEndDate: null,
  hoursPerStoryPoint: 6.5,
  readOnly: false,
  owner: { id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04", fullName: "Ana Pérez" },
  members: [
    {
      userId: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
      email: "ana@example.com",
      isOwner: true,
      joinedAt: "2026-10-01T12:00:00Z",
    },
  ],
  hasActiveSprint: false,
  activeSprint: null,
  backlog: { countsByState: {}, estimatedStoryPoints: 0, estimatedHours: 0 },
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body: ProjectDetail | ErrorResponse | ValidationErrorResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
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
    name: screen.getByLabelText(/^nombre/i),
    description: screen.getByLabelText(/descripción/i),
    startDate: screen.getByLabelText(/fecha de inicio/i),
    plannedEndDate: screen.getByLabelText(/fecha de finalización prevista/i),
    hoursPerStoryPoint: screen.getByLabelText(/horas por story point/i),
    submit: screen.getByRole("button", { name: /crear proyecto/i }),
  };
}

// Completa el formulario con los valores dados y lo envía.
function fillAndSubmit(values: FormValues) {
  const fields = getFields();
  fireEvent.change(fields.name, { target: { value: values.name } });
  fireEvent.change(fields.description, {
    target: { value: values.description },
  });
  fireEvent.change(fields.startDate, { target: { value: values.startDate } });
  fireEvent.change(fields.plannedEndDate, {
    target: { value: values.plannedEndDate },
  });
  fireEvent.change(fields.hoursPerStoryPoint, {
    target: { value: values.hoursPerStoryPoint },
  });
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
  const body = JSON.parse(String(init?.body)) as ProjectInput;
  return { url, method: init?.method, authorization, body };
}

// Envía el formulario con los valores dados y espera a que el campo indicado quede marcado como
// inválido, sin que se haya llamado al backend.
async function expectRejectedBeforeSending(
  fetchMock: FetchMock,
  values: FormValues,
  field: keyof ReturnType<typeof getFields>,
) {
  const fields = fillAndSubmit(values);
  await waitFor(() =>
    expect(fields[field].getAttribute("aria-invalid")).toBe("true"),
  );
  expect(fetchMock).not.toHaveBeenCalled();
  return fields[field];
}

describe("002/US1 - Crear un proyecto", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que responde 201.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(async () =>
      jsonResponse(201, createdProject),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Los límites del nombre, de la descripción y del factor se comunican junto a cada campo desde
  // el inicio, sin haber enviado nada y sin que ningún campo esté marcado como inválido.
  it("muestra antes de enviar los 100 caracteres del nombre, los 1000 de la descripción y el factor de 0,01 a 40 con hasta 2 decimales", () => {
    render(<ProjectForm />);
    const { name, description, hoursPerStoryPoint } = getFields();

    expect(describedText(name)).toMatch(/100/);
    expect(describedText(name)).toMatch(/caracteres/i);
    expect(describedText(description)).toMatch(/1000|1\.000/);
    expect(describedText(description)).toMatch(/caracteres/i);
    const factorHint = describedText(hoursPerStoryPoint);
    expect(factorHint).toMatch(/0[,.]01/);
    expect(factorHint).toMatch(/40/);
    expect(factorHint).toMatch(/2 decimales/i);

    for (const field of [name, description, hoursPerStoryPoint]) {
      expect(field.getAttribute("aria-invalid")).not.toBe("true");
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso normal: el formulario envía POST /api/v1/projects con la sesión de la pestaña y confirma
  // que el proyecto quedó creado en estado Planificado.
  it("Creación exitosa", async () => {
    render(<ProjectForm />);
    fillAndSubmit(validValues);

    expect(await screen.findByText(/proyecto creado/i)).toBeTruthy();
    expect(document.body.textContent).toContain("Portal de Clientes");
    expect(document.body.textContent).toContain("Planificado");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith("/api/v1/projects")).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toEqual({
      name: "Portal de Clientes",
      description: "Rediseño del portal de autogestión.",
      startDate: "2026-10-01",
      plannedEndDate: "2026-12-20",
      hoursPerStoryPoint: 6.5,
    });
  });

  // Caso alternativo: sin descripción el proyecto se envía igual y se crea.
  it("Sin descripción", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(201, { ...createdProject, description: null }),
    );
    render(<ProjectForm />);
    fillAndSubmit({ ...validValues, description: "" });

    expect(await screen.findByText(/proyecto creado/i)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestAt(fetchMock, 0).body.description ?? "").toBe("");
  });

  // Caso alternativo: el formulario no bloquea un nombre que otro usuario ya usa; lo envía y, si
  // el backend lo crea, confirma la creación.
  it("Mismo nombre que el proyecto de otro propietario", async () => {
    render(<ProjectForm />);
    fillAndSubmit(validValues);

    expect(await screen.findByText(/proyecto creado/i)).toBeTruthy();
    expect(requestAt(fetchMock, 0).body.name).toBe("Portal de Clientes");
  });

  // Caso límite: una fecha de finalización prevista igual a la de inicio se envía.
  it("Fechas iguales", async () => {
    render(<ProjectForm />);
    fillAndSubmit({ ...validValues, plannedEndDate: "2026-10-01" });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const { body } = requestAt(fetchMock, 0);
    expect(body.startDate).toBe("2026-10-01");
    expect(body.plannedEndDate).toBe("2026-10-01");
  });

  // Caso límite: los factores 0,01 y 40 se envían como números, sin redondear.
  it("Límites del factor", async () => {
    const factors: Array<[string, number]> = [
      ["0.01", 0.01],
      ["40", 40],
    ];
    for (const [index, [typed, sent]] of factors.entries()) {
      render(<ProjectForm />);
      fillAndSubmit({ ...validValues, hoursPerStoryPoint: typed });
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(index + 1));
      expect(requestAt(fetchMock, index).body.hoursPerStoryPoint).toBe(sent);
      cleanup();
    }
  });

  // Caso límite: un nombre de 100 caracteres se envía y uno de 101 se rechaza antes de enviar
  // indicando el máximo.
  it("Límites del nombre", async () => {
    render(<ProjectForm />);
    fillAndSubmit({ ...validValues, name: "a".repeat(100) });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    cleanup();
    fetchMock.mockClear();

    render(<ProjectForm />);
    const name = await expectRejectedBeforeSending(
      fetchMock,
      { ...validValues, name: "a".repeat(101) },
      "name",
    );
    expect(describedText(name)).toMatch(/100/);
  });

  // FR-003: una descripción de 1001 caracteres se rechaza antes de enviar indicando el máximo.
  it("Límites de la descripción", async () => {
    render(<ProjectForm />);
    const description = await expectRejectedBeforeSending(
      fetchMock,
      { ...validValues, description: "d".repeat(1001) },
      "description",
    );
    expect(describedText(description)).toMatch(/1000|1\.000/);
  });

  // Caso de error: sin nombre, sin fechas y sin factor no se envía nada y cada campo indica qué
  // corregir.
  it("Campos faltantes", async () => {
    render(<ProjectForm />);
    const fields = fillAndSubmit({
      name: "   ",
      description: "",
      startDate: "",
      plannedEndDate: "",
      hoursPerStoryPoint: "",
    });

    await waitFor(() =>
      expect(fields.name.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.name)).toMatch(/nombre/i);
    expect(fields.startDate.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(fields.startDate)).toMatch(/fecha/i);
    expect(fields.plannedEndDate.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(fields.plannedEndDate)).toMatch(/fecha/i);
    expect(fields.hoursPerStoryPoint.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(fields.hoursPerStoryPoint)).toMatch(/factor/i);
    expect(fields.description.getAttribute("aria-invalid")).not.toBe("true");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // FR-010: si el backend rechaza un campo con 400, el mensaje aparece junto a ese campo y no se
  // confirma la creación.
  it("Campos faltantes: muestra el detalle por campo que devuelve el backend", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [
          {
            field: "name",
            message: "El nombre del proyecto es obligatorio.",
          },
        ],
      }),
    );
    render(<ProjectForm />);
    const { name } = fillAndSubmit(validValues);

    await waitFor(() => expect(name.getAttribute("aria-invalid")).toBe("true"));
    expect(describedText(name)).toContain(
      "El nombre del proyecto es obligatorio.",
    );
    expect(screen.queryByText(/proyecto creado/i)).toBeNull();
  });

  // Caso de error: factor 0, negativo, 40,01 o 6,555 se rechaza antes de enviar indicando el rango
  // y la precisión admitidos.
  it("Factor inválido", async () => {
    for (const value of ["0", "-1", "40.01", "6.555"]) {
      render(<ProjectForm />);
      const factor = await expectRejectedBeforeSending(
        fetchMock,
        { ...validValues, hoursPerStoryPoint: value },
        "hoursPerStoryPoint",
      );
      const description = describedText(factor);
      expect(description, `factor ${value}`).toMatch(/40/);
      expect(description, `factor ${value}`).toMatch(/2 decimales/i);
      cleanup();
    }
  });

  // Caso de error: una fecha de finalización prevista anterior a la de inicio se rechaza antes de
  // enviar.
  it("Fecha de finalización prevista anterior a la de inicio", async () => {
    render(<ProjectForm />);
    const plannedEndDate = await expectRejectedBeforeSending(
      fetchMock,
      { ...validValues, plannedEndDate: "2026-09-30" },
      "plannedEndDate",
    );
    expect(describedText(plannedEndDate)).toMatch(/anterior/i);
  });

  // Caso de error: el backend responde 409 y el formulario avisa que ya existe un proyecto propio
  // con ese nombre, sin confirmar la creación.
  it("Nombre repetido", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, {
        error: "Ya existe un proyecto propio con ese nombre.",
      }),
    );
    render(<ProjectForm />);
    fillAndSubmit({ ...validValues, name: "  portal de clientes  " });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(
      "Ya existe un proyecto propio con ese nombre.",
    );
    expect(screen.queryByText(/proyecto creado/i)).toBeNull();
  });

  // Caso de error: si el backend rechaza la sesión con 401, la sesión de la pestaña se descarta
  // para que la guardia pida iniciar sesión, y no se confirma ninguna creación.
  it("Sin sesión", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(401, { error: "La sesión no es válida." }),
    );
    render(<ProjectForm />);
    fillAndSubmit(validValues);

    await waitFor(() => expect(readToken()).toBeNull());
    expect(wasSessionRejected()).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestAt(fetchMock, 0).authorization).toBe(
      `Bearer ${sessionToken}`,
    );
    expect(screen.queryByText(/proyecto creado/i)).toBeNull();
  });
});
