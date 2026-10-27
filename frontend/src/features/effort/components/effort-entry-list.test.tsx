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
  within,
} from "@testing-library/react";
import { EffortEntryList } from "@/features/effort/components/effort-entry-list";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";
import type { EffortConflictResponse, EffortEntry } from "@/types/effort";

// Pruebas de la lista de registros de esfuerzo de una historia (T035). El componente es de
// presentación: recibe los registros tal como los devuelve GET /api/v1/stories/{storyId}/effort
// (entries de StoryEffort en contracts/openapi.yaml), ya ordenados por el backend (FR-035, T034).
//
// Contrato que fijan estas pruebas para la lista:
// - Recibe entries.
// - Los registros se muestran en una lista con nombre accesible "Registros de esfuerzo", un elemento
//   por registro en el orden recibido, con la fecha en DD/MM/AAAA, el nombre completo del autor, la
//   actividad, las horas con formatHours ("6,5 h") y el nombre del sprint, o "Sin sprint" cuando el
//   registro no tiene sprint asociado.
// - Sin registros se muestra el aviso "Todavía no hay esfuerzo registrado." y ningún elemento de
//   lista; no es un error, así que no se usa role="alert".

const ana = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const bruno = {
  id: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9",
  fullName: "Bruno Díaz",
};
const story = {
  id: "2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f",
  title: "Alta de clientes",
};

// Arma un registro de esfuerzo de la historia con los datos que importan en cada prueba.
function entry(
  id: string,
  overrides: Partial<EffortEntry> & Pick<EffortEntry, "entryDate">,
): EffortEntry {
  return {
    id,
    activity: "Desarrollo del formulario",
    hours: 2,
    author: ana,
    story,
    sprint: null,
    definitive: false,
    createdAt: `${overrides.entryDate}T15:00:00Z`,
    updatedAt: `${overrides.entryDate}T15:00:00Z`,
    ...overrides,
  };
}

// Devuelve los elementos de la lista de registros.
function entryItems(): HTMLElement[] {
  const list = screen.getByRole("list", { name: "Registros de esfuerzo" });
  return within(list).queryAllByRole("listitem");
}

afterEach(() => {
  cleanup();
});

// Verificación del detalle de cada registro; complementa el escenario 1 de US2 sin ser uno propio.
describe("EffortEntryList - detalle de cada registro", () => {
  it("cada registro con fecha, autor, actividad, horas y sprint", () => {
    const entries = [
      entry("a1", {
        entryDate: "2026-10-20",
        activity: "Diseño de la pantalla",
        hours: 6.5,
        author: ana,
        sprint: { id: "s1", name: "Sprint 2", status: "active" },
      }),
      entry("b1", {
        entryDate: "2026-10-19",
        activity: "Revisión de código",
        hours: 4,
        author: bruno,
        sprint: { id: "s1", name: "Sprint 2", status: "active" },
      }),
    ];
    render(<EffortEntryList entries={entries} />);

    const items = entryItems();
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("20/10/2026")).toBeTruthy();
    expect(within(items[0]).getByText("Ana Pérez")).toBeTruthy();
    expect(within(items[0]).getByText("Diseño de la pantalla")).toBeTruthy();
    expect(within(items[0]).getByText("6,5 h")).toBeTruthy();
    expect(within(items[0]).getByText("Sprint 2")).toBeTruthy();
    expect(within(items[1]).getByText("19/10/2026")).toBeTruthy();
    expect(within(items[1]).getByText("Bruno Díaz")).toBeTruthy();
    expect(within(items[1]).getByText("Revisión de código")).toBeTruthy();
    expect(within(items[1]).getByText("4 h")).toBeTruthy();
  });
});

describe("006/US2 - Consultar el esfuerzo de una historia", () => {
  it("Caso alternativo — registros de dos sprints distintos y uno sin sprint", () => {
    const entries = [
      entry("e1", {
        entryDate: "2026-10-22",
        activity: "Pruebas de aceptación",
        sprint: { id: "s2", name: "Sprint 3", status: "active" },
      }),
      entry("e2", {
        entryDate: "2026-10-12",
        activity: "Análisis del requisito",
        sprint: { id: "s1", name: "Sprint 2", status: "closed" },
        definitive: true,
      }),
      entry("e3", {
        entryDate: "2026-10-05",
        activity: "Relevamiento inicial",
        sprint: null,
      }),
    ];
    render(<EffortEntryList entries={entries} />);

    const items = entryItems();
    expect(items).toHaveLength(3);
    expect(within(items[0]).getByText("Sprint 3")).toBeTruthy();
    expect(within(items[1]).getByText("Sprint 2")).toBeTruthy();
    expect(within(items[2]).getByText("Sin sprint")).toBeTruthy();
    expect(within(items[0]).queryByText("Sin sprint")).toBeNull();
    expect(within(items[1]).queryByText("Sin sprint")).toBeNull();
  });

  it("Caso alternativo — registros de una persona quitada del proyecto", () => {
    // El registro de la ex integrante llega con su nombre como cualquier otro y se muestra igual.
    const carla = {
      id: "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d",
      fullName: "Carla Ruiz",
    };
    render(
      <EffortEntryList
        entries={[
          entry("c1", {
            entryDate: "2026-10-08",
            activity: "Maquetado",
            author: carla,
          }),
        ]}
      />,
    );

    const items = entryItems();
    expect(items).toHaveLength(1);
    expect(within(items[0]).getByText("Carla Ruiz")).toBeTruthy();
  });

  it("Caso límite — historia sin ningún registro de esfuerzo", () => {
    render(<EffortEntryList entries={[]} />);

    expect(
      screen.getByText("Todavía no hay esfuerzo registrado."),
    ).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("Caso límite — un único registro de 0,25 horas", () => {
    render(
      <EffortEntryList
        entries={[entry("q1", { entryDate: "2026-10-21", hours: 0.25 })]}
      />,
    );

    const items = entryItems();
    expect(items).toHaveLength(1);
    expect(within(items[0]).getByText("0,25 h")).toBeTruthy();
  });

  it("Caso límite — una sola persona con esfuerzo en tres fechas distintas", () => {
    const entries = [
      entry("d3", { entryDate: "2026-10-21", activity: "Tercer día" }),
      entry("d2", { entryDate: "2026-10-20", activity: "Segundo día" }),
      entry("d1", { entryDate: "2026-10-19", activity: "Primer día" }),
    ];
    render(<EffortEntryList entries={entries} />);

    const items = entryItems();
    expect(items).toHaveLength(3);
    expect(within(items[0]).getByText("21/10/2026")).toBeTruthy();
    expect(within(items[1]).getByText("20/10/2026")).toBeTruthy();
    expect(within(items[2]).getByText("19/10/2026")).toBeTruthy();
    expect(
      screen.queryByText("Todavía no hay esfuerzo registrado."),
    ).toBeNull();
  });
});

// Pruebas de la corrección y la eliminación de registros propios desde la lista (T046). Atraviesan
// la lista, la fila editable (T054), las llamadas de api.ts (T054) y el cliente HTTP real; solo se
// sustituye fetch, que es la frontera con el backend. Las reglas de autoría, plazo de gracia, tope
// diario y solo lectura las resuelve el backend: acá se verifica qué ofrece la lista, qué envía y qué
// muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para T054:
// - EffortEntryList recibe además currentUserId (el titular de la sesión) y onChanged, que se llama
//   sin argumentos después de una corrección (200) o una eliminación (204) exitosas, para que la
//   sección vuelva a pedir el total, el desglose y la lista.
// - Los botones "Editar" y "Eliminar" aparecen solo en los registros cuyo author.id es
//   currentUserId y que tienen definitive: false (FR-018, FR-020, FR-022).
// - "Editar" abre la edición en línea dentro del mismo elemento de la lista, con los campos
//   "Fecha", "Actividad" y "Horas" precargados con los valores del registro y los botones
//   "Guardar cambios" y "Cancelar". Cancelar vuelve a la vista sin enviar nada.
// - "Guardar cambios" envía PATCH /api/v1/effort-entries/{id} con la sesión de la pestaña y solo
//   los campos cambiados (EffortEntryPatch), con las horas como número.
// - "Eliminar" no envía nada: pide confirmación con la pregunta "¿Eliminar este registro?" y los
//   botones "Confirmar eliminación" y "Cancelar". Solo "Confirmar eliminación" envía
//   DELETE /api/v1/effort-entries/{id}.
// - Los rechazos 403 y 409 se muestran con role="alert" con el mensaje del backend, sin llamar a
//   onChanged; en la edición, los campos conservan lo escrito.

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";

// Mensajes del backend para los rechazos de la corrección y la eliminación
// (contracts/openapi.yaml y T053).
const notAuthorMessage =
  "Solo quien registró este esfuerzo puede modificarlo o eliminarlo.";
const finalizedMessage =
  "Este esfuerzo pertenece a un sprint cerrado y ya es definitivo.";
const readOnlyMessage =
  "El proyecto está finalizado y su esfuerzo es de solo lectura.";
const capMessage =
  "Superarías las 24 horas registradas para ese día. Te quedan 0 horas disponibles.";

// Registro propio de 4 horas asociado a un sprint Activo (US3, escenario 1).
const ownEntry = entry("e-own", {
  entryDate: "2026-10-20",
  activity: "Desarrollo del formulario",
  hours: 4,
  author: ana,
  sprint: { id: "s1", name: "Sprint 2", status: "active" },
});

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body: EffortEntry | ErrorResponse | EffortConflictResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Responde 200 a un PATCH con el registro corregido a partir del cuerpo recibido, y 204 a un DELETE.
function respondSuccess() {
  return async (_input: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === "DELETE") {
      return new Response(null, { status: 204 });
    }
    const patch = JSON.parse(String(init?.body)) as Partial<EffortEntry>;
    return jsonResponse(200, { ...ownEntry, ...patch });
  };
}

// Extrae la URL, el método, la cabecera Authorization y el cuerpo JSON (si lo hay) de la llamada
// número index a fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const authorization = new Headers(init?.headers).get("Authorization");
  const body =
    init?.body === undefined || init.body === null
      ? undefined
      : (JSON.parse(String(init.body)) as Record<string, unknown>);
  return { url, method: init?.method, authorization, body };
}

// Busca un botón por su nombre dentro de un elemento de la lista.
function buttonIn(item: HTMLElement, name: string): HTMLElement | null {
  return within(item).queryByRole("button", { name });
}

// Abre la edición en línea del primer registro y devuelve sus campos y botones.
function openEdit() {
  const item = entryItems()[0];
  fireEvent.click(within(item).getByRole("button", { name: "Editar" }));
  const row = entryItems()[0];
  return {
    row,
    entryDate: within(row).getByLabelText<HTMLInputElement>(/^fecha/i),
    activity: within(row).getByLabelText<
      HTMLInputElement | HTMLTextAreaElement
    >(/^actividad/i),
    hours: within(row).getByLabelText<HTMLInputElement>(/^horas/i),
    save: within(row).getByRole("button", { name: "Guardar cambios" }),
  };
}

describe("006/US3 - Corregir o eliminar un registro propio", () => {
  let fetchMock: FetchMock;
  let onChanged: Mock<() => void>;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que acepta los cambios.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(respondSuccess());
    vi.stubGlobal("fetch", fetchMock);
    onChanged = vi.fn<() => void>();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Renderiza la lista vista por Ana, titular de la sesión.
  function renderAsAna(entries: EffortEntry[]) {
    render(
      <EffortEntryList
        entries={entries}
        currentUserId={ana.id}
        onChanged={onChanged}
      />,
    );
  }

  // Caso normal: el autor corrige las horas de 4 a 6 y la actividad; la edición en línea parte de
  // los valores del registro y envía solo los dos campos cambiados, nunca autor, historia ni sprint.
  it("Caso normal — corrección de 4 a 6 horas con cambio de actividad", async () => {
    renderAsAna([ownEntry]);
    const fields = openEdit();
    expect(fields.entryDate.value).toBe("2026-10-20");
    expect(fields.activity.value).toBe("Desarrollo del formulario");
    expect(fields.hours.value).toBe("4");

    fireEvent.change(fields.activity, {
      target: { value: "Implementación y pruebas del formulario" },
    });
    fireEvent.change(fields.hours, { target: { value: "6" } });
    fireEvent.click(fields.save);

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(`/api/v1/effort-entries/${ownEntry.id}`)).toBe(
      true,
    );
    expect(request.method).toBe("PATCH");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toEqual({
      activity: "Implementación y pruebas del formulario",
      hours: 6,
    });
  });

  // Caso normal: "Eliminar" pide confirmación sin enviar nada; cancelar la cierra y solo
  // "Confirmar eliminación" envía el DELETE del registro.
  it("Caso normal — eliminación de un registro propio de un sprint Activo", async () => {
    renderAsAna([ownEntry]);
    fireEvent.click(
      within(entryItems()[0]).getByRole("button", { name: "Eliminar" }),
    );
    expect(screen.getByText("¿Eliminar este registro?")).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(screen.queryByText("¿Eliminar este registro?")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();

    fireEvent.click(
      within(entryItems()[0]).getByRole("button", { name: "Eliminar" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Confirmar eliminación" }),
    );

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(`/api/v1/effort-entries/${ownEntry.id}`)).toBe(
      true,
    );
    expect(request.method).toBe("DELETE");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  });

  // Caso alternativo: el autor cambia solo la fecha; se envía únicamente entryDate, y cancelar una
  // edición previa no envía nada ni conserva lo descartado.
  it("Caso alternativo — cambio de la fecha de un registro cargado con la fecha equivocada", async () => {
    renderAsAna([ownEntry]);
    let fields = openEdit();
    fireEvent.change(fields.hours, { target: { value: "8" } });
    fireEvent.click(
      within(fields.row).getByRole("button", { name: "Cancelar" }),
    );
    expect(fetchMock).not.toHaveBeenCalled();
    expect(within(entryItems()[0]).getByText("4 h")).toBeTruthy();

    fields = openEdit();
    expect(fields.hours.value).toBe("4");
    fireEvent.change(fields.entryDate, { target: { value: "2026-10-19" } });
    fireEvent.click(fields.save);

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(requestAt(fetchMock, 0).body).toEqual({ entryDate: "2026-10-19" });
  });

  // Caso límite: un registro de un sprint cerrado hace 10 horas sigue con definitive: false
  // mientras dura el plazo de gracia, así que se puede corregir y eliminar.
  it("Caso límite — registro de un sprint cerrado hace 10 horas con el plazo de gracia vigente", () => {
    renderAsAna([
      entry("e-grace", {
        entryDate: "2026-10-15",
        author: ana,
        sprint: { id: "s0", name: "Sprint 1", status: "closed" },
        definitive: false,
      }),
    ]);

    const item = entryItems()[0];
    expect(buttonIn(item, "Editar")).not.toBeNull();
    expect(buttonIn(item, "Eliminar")).not.toBeNull();
  });

  // Caso límite: subir el registro de 6 a 8 horas en un día completo; el 409 del tope diario se
  // muestra y los campos conservan lo escrito.
  it("Caso límite — aumento que haría superar las 24 horas del día", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, {
        code: "daily_cap_exceeded",
        error: capMessage,
        availableHours: 0,
      }),
    );
    renderAsAna([{ ...ownEntry, hours: 6 }]);
    const fields = openEdit();
    fireEvent.change(fields.hours, { target: { value: "8" } });
    fireEvent.click(fields.save);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(capMessage);
    expect(fields.hours.value).toBe("8");
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: los registros de otro integrante no ofrecen editar ni eliminar, aunque los
  // propios sí; y si el backend igual responde 403 (por ejemplo, con la autoría desactualizada),
  // el mensaje se muestra y lo escrito se conserva.
  it("Caso de error — registro de otro integrante", async () => {
    renderAsAna([
      ownEntry,
      entry("e-bruno", { entryDate: "2026-10-19", author: bruno }),
    ]);
    const [own, other] = entryItems();
    expect(buttonIn(own, "Editar")).not.toBeNull();
    expect(buttonIn(own, "Eliminar")).not.toBeNull();
    expect(buttonIn(other, "Editar")).toBeNull();
    expect(buttonIn(other, "Eliminar")).toBeNull();

    fetchMock.mockImplementation(async () =>
      jsonResponse(403, { error: notAuthorMessage }),
    );
    const fields = openEdit();
    fireEvent.change(fields.activity, { target: { value: "Otra actividad" } });
    fireEvent.click(fields.save);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(notAuthorMessage);
    expect(fields.activity.value).toBe("Otra actividad");
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: un registro propio ya definitivo no ofrece editar ni eliminar; si el plazo vence
  // con la edición abierta, el 409 se muestra y lo escrito se conserva.
  it("Caso de error — modificación de un registro de un sprint cerrado con el plazo vencido", async () => {
    renderAsAna([
      ownEntry,
      entry("e-final", {
        entryDate: "2026-10-05",
        author: ana,
        sprint: { id: "s0", name: "Sprint 1", status: "closed" },
        definitive: true,
      }),
    ]);
    const definitive = entryItems()[1];
    expect(buttonIn(definitive, "Editar")).toBeNull();
    expect(buttonIn(definitive, "Eliminar")).toBeNull();

    fetchMock.mockImplementation(async () =>
      jsonResponse(409, {
        code: "effort_finalized",
        error: finalizedMessage,
        availableHours: null,
      }),
    );
    const fields = openEdit();
    fireEvent.change(fields.hours, { target: { value: "5" } });
    fireEvent.click(fields.save);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(finalizedMessage);
    expect(fields.hours.value).toBe("5");
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: si la eliminación confirmada se rechaza con 409 porque el registro ya es
  // definitivo, el mensaje se muestra y el registro sigue en la lista.
  it("Caso de error — eliminación de un registro de un sprint cerrado con el plazo vencido", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, {
        code: "effort_finalized",
        error: finalizedMessage,
        availableHours: null,
      }),
    );
    renderAsAna([ownEntry]);
    fireEvent.click(
      within(entryItems()[0]).getByRole("button", { name: "Eliminar" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Confirmar eliminación" }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(finalizedMessage);
    expect(entryItems()).toHaveLength(1);
    expect(
      within(entryItems()[0]).getByText("Desarrollo del formulario"),
    ).toBeTruthy();
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: un registro propio sin sprint asociado nunca es definitivo y se puede corregir
  // y eliminar.
  it("Caso de error — registro propio sin sprint asociado", () => {
    renderAsAna([
      entry("e-nosprint", {
        entryDate: "2026-10-05",
        author: ana,
        sprint: null,
      }),
    ]);

    const item = entryItems()[0];
    expect(buttonIn(item, "Editar")).not.toBeNull();
    expect(buttonIn(item, "Eliminar")).not.toBeNull();
  });

  // Caso de error: en un proyecto Finalizado el backend responde 409 de solo lectura; el mensaje se
  // muestra y lo escrito se conserva.
  it("Caso de error — proyecto Finalizado", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, {
        code: "read_only",
        error: readOnlyMessage,
        availableHours: null,
      }),
    );
    renderAsAna([ownEntry]);
    const fields = openEdit();
    fireEvent.change(fields.activity, { target: { value: "Ajuste final" } });
    fireEvent.click(fields.save);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(readOnlyMessage);
    expect(fields.activity.value).toBe("Ajuste final");
    expect(onChanged).not.toHaveBeenCalled();
  });
});
