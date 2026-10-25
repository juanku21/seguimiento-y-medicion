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
import { EffortEntryForm } from "@/features/effort/components/effort-entry-form";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";

// Pruebas del formulario de registro de esfuerzo (T018). Atraviesan el componente, la validación del
// formulario y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el backend, y el
// reloj del navegador, para fijar "hoy".
//
// Contrato de accesibilidad que fijan estas pruebas para el formulario, igual que en la 001 a la 004:
// - Cada campo se encuentra por su etiqueta: "Fecha" (campo de tipo fecha con valor AAAA-MM-DD),
//   "Actividad" y "Horas" (campo numérico con paso 0.25). No hay ningún otro campo: el autor no es
//   un dato de la entrada (FR-002).
// - El envío se hace con el botón "Registrar esfuerzo".
// - Los errores de cada campo, propios o del 400 del backend, se asocian al campo con
//   aria-describedby; un campo con error lleva aria-invalid="true".
// - Los rechazos generales (409 del tope diario, del sprint definitivo y del proyecto de solo
//   lectura, 404 de historia inexistente) se muestran con role="alert" con el mensaje del backend.
// - El alta exitosa se informa con onCreated y el registro devuelto por el backend.

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

// Cuerpo de POST /api/v1/stories/{storyId}/effort-entries según EffortEntryInput de
// contracts/openapi.yaml. Se declara acá para leer lo que envía el formulario sin depender de los
// tipos de T032.
type SentEffortInput = {
  entryDate: string;
  activity: string;
  hours: number;
};

// Respuesta 201 según EffortEntry de contracts/openapi.yaml.
type CreatedEntry = {
  id: string;
  entryDate: string;
  activity: string;
  hours: number;
  author: { id: string; fullName: string };
  story: { id: string; title: string };
  sprint: {
    id: string;
    name: string;
    status: "planned" | "active" | "closed";
  } | null;
  definitive: boolean;
  createdAt: string;
  updatedAt: string;
};

// Rechazo 409 según EffortConflictResponse de contracts/openapi.yaml.
type EffortConflictResponse = {
  code: "read_only" | "effort_finalized" | "daily_cap_exceeded";
  error: string;
  availableHours: number | null;
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const storyId = "2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f";
const entriesPath = `/api/v1/stories/${storyId}/effort-entries`;

// "Hoy" en la zona del navegador: sábado 24/10/2026 a las 10:00 locales. Se arma con la hora local
// para que el día esperado no dependa de la zona de la máquina que corre las pruebas.
const today = "2026-10-24";
const tomorrow = "2026-10-25";
const now = new Date(2026, 9, 24, 10, 0, 0);

// Datos del caso normal de la spec (US1, escenario 1).
const normalActivity = "Implementación del formulario de alta";
const normalHours = "6.5";

const createdEntry: CreatedEntry = {
  id: "7d8e9f0a-1b2c-4d3e-8f4a-5b6c7d8e9f0a",
  entryDate: today,
  activity: normalActivity,
  hours: 6.5,
  author: { id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d", fullName: "Ana Pérez" },
  story: { id: storyId, title: "Alta de clientes" },
  sprint: {
    id: "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
    name: "Sprint 3",
    status: "active",
  },
  definitive: false,
  createdAt: "2026-10-24T13:00:00Z",
  updatedAt: "2026-10-24T13:00:00Z",
};

// Mensajes del backend para los rechazos que el formulario muestra tal como llegan
// (contracts/openapi.yaml y T029).
const capMessage =
  "Superarías las 24 horas registradas para ese día. Te quedan 4 horas disponibles.";
const finalizedMessage =
  "Este esfuerzo pertenece a un sprint cerrado y ya es definitivo.";
const readOnlyMessage =
  "El proyecto está finalizado y su esfuerzo es de solo lectura.";
const futureMessage = "No se registran horas en una fecha futura.";
const beforeStartMessage =
  "La fecha no puede ser anterior al inicio del proyecto (AAAA-MM-DD).";

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body:
    | CreatedEntry
    | ErrorResponse
    | ValidationErrorResponse
    | EffortConflictResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Responde 201 con el registro que el backend armaría a partir del cuerpo recibido, atribuido al
// titular de la sesión.
function respondCreated() {
  return async (_input: RequestInfo | URL, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body)) as SentEffortInput;
    return jsonResponse(201, {
      ...createdEntry,
      entryDate: body.entryDate,
      activity: body.activity.trim(),
      hours: body.hours,
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
    entryDate: screen.getByLabelText<HTMLInputElement>(/^fecha/i),
    activity: screen.getByLabelText<HTMLInputElement | HTMLTextAreaElement>(
      /^actividad/i,
    ),
    hours: screen.getByLabelText<HTMLInputElement>(/^horas/i),
    submit: screen.getByRole("button", { name: /registrar esfuerzo/i }),
  };
}

type FormValues = { entryDate?: string; activity: string; hours: string };

// Completa el formulario y lo envía. La fecha solo se cambia si se indica, para dejar la precargada;
// un campo vacío se deja sin tocar, como cuando la persona no lo completa.
function fillAndSubmit(values: FormValues) {
  const fields = getFields();
  if (values.entryDate !== undefined) {
    fireEvent.change(fields.entryDate, { target: { value: values.entryDate } });
  }
  if (values.activity !== "") {
    fireEvent.change(fields.activity, { target: { value: values.activity } });
  }
  if (values.hours !== "") {
    fireEvent.change(fields.hours, { target: { value: values.hours } });
  }
  fireEvent.click(fields.submit);
  return fields;
}

// Extrae la URL, el método, la cabecera Authorization y el cuerpo JSON de la llamada número index
// a fetch. El cuerpo se devuelve como registro genérico para poder verificar sus claves.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const authorization = new Headers(init?.headers).get("Authorization");
  const body = JSON.parse(String(init?.body)) as Record<string, unknown>;
  return { url, method: init?.method, authorization, body };
}

// Espera a que el campo dado quede marcado como inválido con el mensaje indicado, sin que se haya
// llamado al backend ni avisado un alta.
async function expectRejectedBeforeSending(
  fetchMock: FetchMock,
  onCreated: Mock,
  field: HTMLElement,
  message: string,
) {
  await waitFor(() => expect(field.getAttribute("aria-invalid")).toBe("true"));
  expect(describedText(field)).toContain(message);
  expect(fetchMock).not.toHaveBeenCalled();
  expect(onCreated).not.toHaveBeenCalled();
}

// Envía el formulario con los valores dados y espera que se informe el alta.
async function submitAndExpectCreated(onCreated: Mock, values: FormValues) {
  render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
  fillAndSubmit(values);
  await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
}

// Envía el formulario normal con el backend respondiendo un rechazo general y espera el alerta con
// el mensaje recibido, sin que se informe un alta.
async function expectGeneralRejection(
  fetchMock: FetchMock,
  onCreated: Mock,
  status: number,
  body: ErrorResponse | EffortConflictResponse,
) {
  fetchMock.mockImplementation(async () => jsonResponse(status, body));
  render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
  fillAndSubmit({ activity: normalActivity, hours: normalHours });

  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toContain(body.error);
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(onCreated).not.toHaveBeenCalled();
}

describe("006/US1 - Registrar el esfuerzo trabajado", () => {
  let fetchMock: FetchMock;
  let onCreated: Mock;

  // Cada prueba arranca con una sesión válida en la pestaña, un fetch que responde 201 y el reloj
  // fijo; solo se simula la fecha, para que las esperas de Testing Library sigan funcionando.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(now);
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(respondCreated());
    vi.stubGlobal("fetch", fetchMock);
    onCreated = vi.fn();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  // Caso normal: con la fecha de hoy ya cargada, se escribe la actividad y las horas y se confirma
  // (SC-001: tres pasos). El formulario envía POST con la sesión de la pestaña y solo fecha,
  // actividad y horas como número, y avisa el alta con el registro del backend.
  it("Caso normal — registro de 6,5 horas sobre una historia del sprint Activo", async () => {
    render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
    const fields = getFields();
    expect(fields.entryDate.type).toBe("date");
    expect(fields.entryDate.value).toBe(today);
    expect(fields.hours.type).toBe("number");
    expect(fields.hours.step).toBe("0.25");

    fillAndSubmit({ activity: normalActivity, hours: normalHours });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(onCreated).toHaveBeenCalledWith(createdEntry);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(entriesPath)).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toEqual({
      entryDate: today,
      activity: normalActivity,
      hours: 6.5,
    });
  });

  // Caso límite: 24 horas, el máximo, se envían sin rechazo del formulario.
  it("Caso límite — exactamente 24 horas en una fecha sin horas cargadas", async () => {
    await submitAndExpectCreated(onCreated, {
      activity: normalActivity,
      hours: "24",
    });
    expect(requestAt(fetchMock, 0).body.hours).toBe(24);
  });

  // Caso límite: 0,25 horas, el mínimo, se envían sin rechazo del formulario.
  it("Caso límite — 0,25 horas, el mínimo de la escala", async () => {
    await submitAndExpectCreated(onCreated, {
      activity: normalActivity,
      hours: "0.25",
    });
    expect(requestAt(fetchMock, 0).body.hours).toBe(0.25);
  });

  // Caso límite: una actividad de 300 caracteres se envía y una de 301 se rechaza antes de enviar,
  // con el máximo indicado junto al campo.
  it("Caso límite — actividad de 300 y de 301 caracteres", async () => {
    await submitAndExpectCreated(onCreated, {
      activity: "á".repeat(300),
      hours: normalHours,
    });
    expect(requestAt(fetchMock, 0).body.activity).toBe("á".repeat(300));
    cleanup();
    fetchMock.mockClear();
    onCreated.mockClear();

    render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
    const { activity } = fillAndSubmit({
      activity: "á".repeat(301),
      hours: normalHours,
    });
    await expectRejectedBeforeSending(
      fetchMock,
      onCreated,
      activity,
      "La actividad admite como máximo 300 caracteres.",
    );
  });

  // Caso límite: la fecha precargada es la de hoy en la zona del navegador aun a las 23:30, cuando
  // en UTC ya puede ser el día siguiente; se envía tal cual.
  it("Caso límite — la fecha de hoy según la zona horaria del sistema", async () => {
    vi.setSystemTime(new Date(2026, 9, 24, 23, 30, 0));
    render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
    expect(getFields().entryDate.value).toBe(today);

    fillAndSubmit({ activity: normalActivity, hours: normalHours });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(requestAt(fetchMock, 0).body.entryDate).toBe(today);
  });

  // Caso de error: con el tope diario superado, el backend responde 409 y el formulario muestra el
  // mensaje con las horas que quedan disponibles para ese día.
  it("Caso de error — tope diario superado", async () => {
    await expectGeneralRejection(fetchMock, onCreated, 409, {
      code: "daily_cap_exceeded",
      error: capMessage,
      availableHours: 4,
    });
    expect(screen.getByRole("alert").textContent).toContain(
      "Te quedan 4 horas disponibles.",
    );
  });

  // Caso de error: 0, un negativo o más de 24 horas se rechazan antes de enviar, con el rango
  // admitido junto al campo de horas.
  it("Caso de error — horas en 0, negativas o mayores que 24", async () => {
    for (const hours of ["0", "-1", "24.25"]) {
      render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
      const fields = fillAndSubmit({ activity: normalActivity, hours });
      await expectRejectedBeforeSending(
        fetchMock,
        onCreated,
        fields.hours,
        "Las horas deben ser mayores que 0 y como máximo 24.",
      );
      cleanup();
    }
  });

  // Caso de error: 1,3 horas se rechazan antes de enviar, con el múltiplo de 0,25 junto al campo.
  it("Caso de error — horas que no son múltiplo de 0,25", async () => {
    render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
    const fields = fillAndSubmit({ activity: normalActivity, hours: "1.3" });
    await expectRejectedBeforeSending(
      fetchMock,
      onCreated,
      fields.hours,
      "Las horas se cargan en múltiplos de 0,25 (un cuarto de hora).",
    );
  });

  // Caso de error: sin actividad o con una compuesta solo por espacios no se envía nada y el campo
  // indica que es obligatoria.
  it("Caso de error — actividad ausente o compuesta solo por espacios", async () => {
    for (const activity of ["", "   "]) {
      render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
      const fields = fillAndSubmit({ activity, hours: normalHours });
      await expectRejectedBeforeSending(
        fetchMock,
        onCreated,
        fields.activity,
        "La actividad es obligatoria.",
      );
      cleanup();
    }
  });

  // Caso de error: la fecha futura la resuelve el backend con la zona del sistema; su 400 en
  // entryDate se muestra junto al campo de fecha.
  it("Caso de error — fecha de mañana", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [{ field: "entryDate", message: futureMessage }],
      }),
    );
    render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
    const fields = fillAndSubmit({
      entryDate: tomorrow,
      activity: normalActivity,
      hours: normalHours,
    });

    await waitFor(() =>
      expect(fields.entryDate.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.entryDate)).toContain(futureMessage);
    expect(requestAt(fetchMock, 0).body.entryDate).toBe(tomorrow);
    expect(onCreated).not.toHaveBeenCalled();
  });

  // Caso de error: la fecha anterior al inicio del proyecto la resuelve el backend; su 400 en
  // entryDate se muestra junto al campo de fecha, y si el 400 informa varios campos, cada mensaje
  // queda junto a su campo.
  it("Caso de error — fecha anterior al inicio del proyecto", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [
          { field: "entryDate", message: beforeStartMessage },
          { field: "activity", message: "La actividad es obligatoria." },
          {
            field: "hours",
            message: "Las horas deben ser mayores que 0 y como máximo 24.",
          },
        ],
      }),
    );
    render(<EffortEntryForm storyId={storyId} onCreated={onCreated} />);
    const fields = fillAndSubmit({
      entryDate: "2026-09-30",
      activity: normalActivity,
      hours: normalHours,
    });

    await waitFor(() =>
      expect(fields.entryDate.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.entryDate)).toContain(beforeStartMessage);
    expect(fields.activity.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(fields.activity)).toContain(
      "La actividad es obligatoria.",
    );
    expect(fields.hours.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(fields.hours)).toContain(
      "Las horas deben ser mayores que 0 y como máximo 24.",
    );
    expect(onCreated).not.toHaveBeenCalled();
  });

  // Caso de error: con el plazo de gracia vencido, el backend responde 409 y el formulario indica
  // que el sprint ya es definitivo.
  it("Caso de error — historia Completada de un sprint cerrado hace cinco días", async () => {
    await expectGeneralRejection(fetchMock, onCreated, 409, {
      code: "effort_finalized",
      error: finalizedMessage,
      availableHours: null,
    });
  });

  // Caso de error: en un proyecto Finalizado el backend responde 409 y el formulario indica que el
  // proyecto es de solo lectura.
  it("Caso de error — proyecto Finalizado con el plazo de gracia de su último sprint vigente", async () => {
    await expectGeneralRejection(fetchMock, onCreated, 409, {
      code: "read_only",
      error: readOnlyMessage,
      availableHours: null,
    });
  });

  // Caso de error: quien no es integrante recibe 404 de historia inexistente y el formulario lo
  // muestra sin avisar un alta.
  it("Caso de error — usuario que no es integrante del proyecto", async () => {
    await expectGeneralRejection(fetchMock, onCreated, 404, {
      error: "La historia no existe.",
    });
  });

  // Caso de error: el formulario no ofrece forma de indicar a otra persona como autor; sus únicos
  // campos son fecha, actividad y horas, y el cuerpo enviado no lleva ningún dato de autoría.
  it("Caso de error — intento de indicar a otra persona como autor", async () => {
    const { container } = render(
      <EffortEntryForm storyId={storyId} onCreated={onCreated} />,
    );
    expect(
      screen.queryByLabelText(/autor|integrante|persona|usuario/i),
    ).toBeNull();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(container.querySelectorAll("input, textarea, select")).toHaveLength(
      3,
    );

    fillAndSubmit({ activity: normalActivity, hours: normalHours });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(Object.keys(requestAt(fetchMock, 0).body).sort()).toEqual([
      "activity",
      "entryDate",
      "hours",
    ]);
  });
});
