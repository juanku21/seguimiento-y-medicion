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
import { useState } from "react";
import { DefectDetail } from "@/features/defects/components/defect-detail";
import { DefectStatusActions } from "@/features/defects/components/defect-status-actions";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type {
  Defect,
  DefectConflictResponse,
  DefectSprintRef,
  DefectStatus,
} from "@/types/defect";
import type { SprintListItem, SprintStatus } from "@/types/sprint";

// Pruebas de las acciones de estado de un defecto (T030). Atraviesan el componente, el formulario
// de resolución, las llamadas de api.ts (T037) y el cliente HTTP real; solo se sustituye fetch, que
// es la frontera con el backend. Las reglas de la transición (RN-12 a RN-14, FR-023 a FR-026) las
// resuelve el backend: acá se verifica qué acciones se ofrecen según el estado, qué se envía y qué
// se muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para DefectStatusActions:
// - Recibe el defecto (Defect) y onChanged, que recibe el defecto con su estado vigente para que la
//   pantalla de detalle lo muestre.
// - Las acciones se derivan del estado vigente con allowedTransitions. Un defecto Abierto ofrece el
//   botón "Pasar a En progreso" y no ofrece "Resolver"; uno En progreso ofrece "Resolver" y no
//   ofrece "Pasar a En progreso". Las pruebas no fijan la cantidad total de botones: US5 y US6
//   suman "Reabrir" y "Descartar".
// - "Pasar a En progreso" envía PUT /api/v1/defects/{defectId}/status con la sesión de la pestaña y
//   el cuerpo {"status":"in_progress"}.
// - "Resolver" muestra el selector "Sprint de resolución" con los sprints Activos y Cerrados del
//   proyecto del defecto (nunca uno Planificado; FR-024), con una primera opción vacía sin valor, y
//   el botón "Confirmar resolución", que no se deshabilita por falta de sprint. Confirmar envía
//   {"status":"resolved","resolvedSprintId":"<id>"}; sin sprint elegido no envía resolvedSprintId.
// - Con un 200 llama una vez a onChanged con el defecto recibido.
// - El 400 con field "resolvedSprintId", o el mismo mensaje validado en el cliente, se muestra junto
//   al selector con aria-describedby y aria-invalid="true", sin llamar a onChanged.
// - Los rechazos generales (404 y 409) se muestran con role="alert" con el mensaje del backend.
// - Ante un 409 invalid_transition, además, llama a onChanged con el defecto refrescado, cuyo estado
//   es el currentStatus recibido, y el mensaje sigue visible cuando la pantalla muestra ese estado.

// Sustituye el enrutador de Next.js por si el componente o la guardia de sesión lo usan: fuera del
// App Router no hay enrutador montado.
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    refresh: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(""),
}));

type FetchMock = Mock<typeof fetch>;
type ChangedMock = Mock<(defect: Defect) => void>;
type StatusBody = Record<string, unknown>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const defectId = "7d8e9f0a-1b2c-4d3e-8f4a-5b6c7d8e9f0a";
const statusPath = `/api/v1/defects/${defectId}/status`;

const startButtonName = /pasar a en progreso/i;
const resolveButtonName = /^resolver$/i;
const confirmButtonName = /confirmar resoluci[oó]n/i;
const resolutionSelectLabel = /^sprint de resoluci[oó]n/i;

// Mensajes de contracts/openapi.yaml y de las pruebas de integración del backend (T028).
const resolutionRequiredMessage =
  "El sprint de resolución es obligatorio para resolver.";
const resolutionBeforeMessage =
  "El sprint de resolución no puede empezar antes que el de detección.";
const sprintNotFoundMessage = "El sprint no existe.";
const readOnlyMessage =
  "El proyecto está finalizado y sus defectos son de solo lectura.";
const concurrentStartMessage =
  "No se puede pasar de En progreso a En progreso. Desde En progreso se permite: Resuelto, Descartado.";

// Construye un sprint del listado de la 004 con los datos mínimos que usa el selector.
function sprint(
  id: string,
  name: string,
  status: SprintStatus,
  startDate: string,
): SprintListItem {
  return {
    id,
    name,
    goal: "Objetivo",
    status,
    startDate,
    endDate: startDate,
    startedAt: null,
    closedAt: null,
    storyCount: 0,
    results: null,
  };
}

const firstSprint = sprint(
  "c3333333-0000-4000-8000-000000000003",
  "Sprint 1",
  "closed",
  "2026-09-01",
);
const secondSprint = sprint(
  "c2222222-0000-4000-8000-000000000002",
  "Sprint 2",
  "closed",
  "2026-09-15",
);
const activeSprint = sprint(
  "a1111111-0000-4000-8000-000000000001",
  "Sprint 3",
  "active",
  "2026-10-01",
);
const plannedSprint = sprint(
  "b4444444-0000-4000-8000-000000000004",
  "Sprint 4",
  "planned",
  "2026-10-15",
);
const projectSprints = [activeSprint, secondSprint, firstSprint, plannedSprint];

// Referencia de sprint del defecto (SprintRef de la spec 007) a partir de un sprint del listado.
function sprintRef(item: SprintListItem): DefectSprintRef {
  return {
    id: item.id,
    name: item.name,
    status: item.status,
    startDate: item.startDate,
  };
}

const author = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const actor = {
  id: "1e2d3c4b-5a6f-4e7d-9c8b-0a1b2c3d4e5f",
  fullName: "Bruno Gómez",
};

// Defecto detectado en el segundo sprint del proyecto, en el estado indicado (Defect de
// contracts/openapi.yaml).
function defect(
  status: DefectStatus,
  resolvedSprint: DefectSprintRef | null = null,
): Defect {
  const changed = status === "open" ? author : actor;
  return {
    id: defectId,
    projectId,
    description: "El total del reporte no suma los descuentos",
    severity: "high",
    status,
    story: {
      id: "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10",
      title: "Reporte de ventas mensual",
    },
    detectedSprint: sprintRef(secondSprint),
    resolvedSprint,
    createdBy: author,
    updatedBy: changed,
    statusChangedAt: "2026-10-24T13:00:00Z",
    statusChangedBy: changed,
    createdAt: "2026-10-20T10:00:00Z",
    updatedAt: "2026-10-24T13:00:00Z",
  };
}

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body:
    | Defect
    | ErrorResponse
    | ValidationErrorResponse
    | DefectConflictResponse
    | Record<string, unknown>,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Respuesta 200 del cambio de estado: el defecto con el estado pedido y, al resolver, el sprint
// elegido.
function accepted(body: StatusBody): Response {
  const status = body.status as DefectStatus;
  const chosen = projectSprints.find(
    (item) => item.id === body.resolvedSprintId,
  );
  return jsonResponse(
    200,
    defect(status, chosen === undefined ? null : sprintRef(chosen)),
  );
}

// Backend simulado: devuelve los sprints que cumplen el filtro status de la consulta (como lo hace
// la 004), el defecto con el estado que tiene en el servidor y responde el cambio de estado con
// onStatus. Las pruebas reemplazan onStatus cuando verifican un rechazo.
function backend(
  serverStatus: DefectStatus,
  onStatus: (body: StatusBody) => Response = accepted,
) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, "http://localhost");
    if (init?.method === "PUT" && url.pathname.endsWith(statusPath)) {
      return onStatus(JSON.parse(String(init.body)) as StatusBody);
    }
    if (url.pathname.endsWith("/sprints")) {
      const wanted = url.searchParams.getAll("status");
      const listed = projectSprints.filter(
        (item) => wanted.length === 0 || wanted.includes(item.status),
      );
      return jsonResponse(200, {
        sprints: listed,
        appliedFilters: { status: wanted },
        emptyReason: listed.length === 0 ? "no_matches" : null,
      });
    }
    if (url.pathname.endsWith(`/api/v1/defects/${defectId}`)) {
      return jsonResponse(200, defect(serverStatus));
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Pantalla mínima que muestra el defecto vigente, como el detalle: guarda lo que llega por
// onChanged y se lo vuelve a pasar a las acciones.
function DetailHarness({
  initial,
  onChanged,
}: {
  initial: Defect;
  onChanged: ChangedMock;
}) {
  const [current, setCurrent] = useState(initial);
  return (
    <DefectStatusActions
      defect={current}
      onChanged={(changed) => {
        onChanged(changed);
        setCurrent(changed);
      }}
    />
  );
}

// Monta las acciones dentro del proveedor de sesión, como en el layout raíz.
function renderActions(initial: Defect) {
  const onChanged: ChangedMock = vi.fn();
  render(
    <SessionProvider>
      <DetailHarness initial={initial} onChanged={onChanged} />
    </SessionProvider>,
  );
  return onChanged;
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

// Nombres de las opciones con valor de un select, sin la opción vacía inicial.
function optionNames(select: HTMLSelectElement): string[] {
  return Array.from(select.options)
    .filter((option) => option.value !== "")
    .map((option) => option.textContent ?? "");
}

// Pulsa "Resolver" y espera a que el selector cargue los sprints Activos y Cerrados.
async function openResolution(): Promise<HTMLSelectElement> {
  fireEvent.click(
    await screen.findByRole("button", { name: resolveButtonName }),
  );
  const select = await screen.findByLabelText<HTMLSelectElement>(
    resolutionSelectLabel,
  );
  await waitFor(() => expect(optionNames(select)).toHaveLength(3));
  return select;
}

// Elige el sprint de resolución (o la opción vacía) y confirma.
function confirmResolution(select: HTMLSelectElement, sprintId: string) {
  fireEvent.change(select, { target: { value: sprintId } });
  fireEvent.click(screen.getByRole("button", { name: confirmButtonName }));
}

// Extrae la URL, la cabecera Authorization y el cuerpo JSON de cada cambio de estado enviado.
function statusRequests(fetchMock: FetchMock) {
  return fetchMock.mock.calls
    .filter(([, init]) => init?.method === "PUT")
    .map(([input, init]) => ({
      url:
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.href
            : input.url,
      authorization: new Headers(init?.headers).get("Authorization"),
      body: JSON.parse(String(init?.body)) as StatusBody,
    }));
}

describe("007/US2 - Seguir y resolver un defecto", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y el backend simulado que acepta el
  // cambio de estado.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(backend("open"));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Un defecto Abierto solo puede avanzar a En progreso: no se ofrece resolverlo ni el selector de
  // sprint (US2, escenario 10).
  it("un defecto Abierto ofrece pasar a En progreso y no ofrece resolver", async () => {
    renderActions(defect("open"));

    expect(
      await screen.findByRole("button", { name: startButtonName }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: resolveButtonName }),
    ).toBeNull();
    expect(screen.queryByLabelText(resolutionSelectLabel)).toBeNull();
  });

  // Caso normal: el defecto Abierto pasa a En progreso y después se resuelve en el sprint Activo;
  // cada paso envía solo lo que corresponde y la pantalla muestra el estado recibido (US2,
  // escenario 1).
  it("Abierto pasa a En progreso y más tarde a Resuelto con el sprint Activo", async () => {
    const onChanged = renderActions(defect("open"));

    fireEvent.click(
      await screen.findByRole("button", { name: startButtonName }),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(onChanged).toHaveBeenLastCalledWith(defect("in_progress"));
    const [start] = statusRequests(fetchMock);
    expect(start.url.endsWith(statusPath)).toBe(true);
    expect(start.authorization).toBe(`Bearer ${sessionToken}`);
    expect(start.body).toEqual({ status: "in_progress" });

    const select = await openResolution();
    confirmResolution(select, activeSprint.id);
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(2));
    expect(onChanged).toHaveBeenLastCalledWith(
      defect("resolved", sprintRef(activeSprint)),
    );
    expect(statusRequests(fetchMock)[1]?.body).toEqual({
      status: "resolved",
      resolvedSprintId: activeSprint.id,
    });
  });

  // Un defecto En progreso ofrece resolver y no volver a En progreso; el selector ofrece los
  // sprints Activos y Cerrados del proyecto, nunca uno Planificado, y un Cerrado sirve como sprint
  // de resolución (US2, escenarios 5 y 9).
  it("un defecto En progreso ofrece Resolver con los sprints Activos y Cerrados", async () => {
    const onChanged = renderActions(defect("in_progress"));

    expect(
      await screen.findByRole("button", { name: resolveButtonName }),
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: startButtonName })).toBeNull();

    const select = await openResolution();
    expect([...optionNames(select)].sort()).toEqual([
      "Sprint 1",
      "Sprint 2",
      "Sprint 3",
    ]);
    expect(optionNames(select)).not.toContain(plannedSprint.name);

    confirmResolution(select, secondSprint.id);
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(statusRequests(fetchMock)[0]?.body).toEqual({
      status: "resolved",
      resolvedSprintId: secondSprint.id,
    });
  });

  // Caso de error: confirmar sin sprint de resolución muestra que es obligatorio junto al selector
  // y el defecto sigue En progreso (US2, escenario 7).
  it("resolver sin sprint de resolución muestra el mensaje junto al selector", async () => {
    fetchMock.mockImplementation(
      backend("in_progress", () =>
        jsonResponse(400, {
          error: "Hay campos inválidos en la solicitud.",
          fields: [
            { field: "resolvedSprintId", message: resolutionRequiredMessage },
          ],
        }),
      ),
    );
    const onChanged = renderActions(defect("in_progress"));

    const select = await openResolution();
    confirmResolution(select, "");

    await waitFor(() =>
      expect(select.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(select)).toContain(resolutionRequiredMessage);
    expect(onChanged).not.toHaveBeenCalled();
    for (const request of statusRequests(fetchMock)) {
      expect(request.body).not.toHaveProperty("resolvedSprintId");
    }
  });

  // Caso de error: el 400 del backend por un sprint que empieza antes que el de detección se
  // muestra junto al selector, sin cambiar el defecto (US2, escenario 8).
  it("el 400 del sprint de resolución se muestra junto al selector", async () => {
    fetchMock.mockImplementation(
      backend("in_progress", () =>
        jsonResponse(400, {
          error: "Hay campos inválidos en la solicitud.",
          fields: [
            { field: "resolvedSprintId", message: resolutionBeforeMessage },
          ],
        }),
      ),
    );
    const onChanged = renderActions(defect("in_progress"));

    const select = await openResolution();
    confirmResolution(select, firstSprint.id);

    await waitFor(() =>
      expect(select.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(select)).toContain(resolutionBeforeMessage);
    expect(statusRequests(fetchMock)[0]?.body).toEqual({
      status: "resolved",
      resolvedSprintId: firstSprint.id,
    });
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: otro integrante ya pasó el defecto a En progreso. El 409 invalid_transition
  // muestra el mensaje y la pantalla se refresca con el currentStatus recibido, que ya ofrece
  // resolver (FR-018, RN-24).
  it("ante un 409 invalid_transition muestra el mensaje y refresca el defecto con el currentStatus", async () => {
    fetchMock.mockImplementation(
      backend("in_progress", () =>
        jsonResponse(409, {
          code: "invalid_transition",
          error: concurrentStartMessage,
          currentStatus: "in_progress",
          allowedStatuses: ["resolved", "discarded"],
        }),
      ),
    );
    const onChanged = renderActions(defect("open"));

    fireEvent.click(
      await screen.findByRole("button", { name: startButtonName }),
    );

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(onChanged).toHaveBeenCalledWith(
      expect.objectContaining({ id: defectId, status: "in_progress" }),
    );
    expect(
      await screen.findByRole("button", { name: resolveButtonName }),
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: startButtonName })).toBeNull();
    expect((await screen.findByRole("alert")).textContent).toContain(
      concurrentStartMessage,
    );
  });

  // Caso de error: un sprint de resolución de otro proyecto se rechaza como sprint inexistente y
  // el defecto no cambia (US2, escenario 12).
  it("un sprint de resolución de otro proyecto muestra que el sprint no existe", async () => {
    fetchMock.mockImplementation(
      backend("in_progress", () =>
        jsonResponse(404, { error: sprintNotFoundMessage }),
      ),
    );
    const onChanged = renderActions(defect("in_progress"));

    const select = await openResolution();
    confirmResolution(select, activeSprint.id);

    expect((await screen.findByRole("alert")).textContent).toContain(
      sprintNotFoundMessage,
    );
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: en un proyecto Finalizado el backend responde 409 read_only; se muestra el
  // mensaje de solo lectura y el defecto no cambia (US2, escenario 13).
  it("el proyecto Finalizado muestra el mensaje de solo lectura", async () => {
    fetchMock.mockImplementation(
      backend("open", () =>
        jsonResponse(409, {
          code: "read_only",
          error: readOnlyMessage,
          currentStatus: null,
          allowedStatuses: [],
        }),
      ),
    );
    const onChanged = renderActions(defect("open"));

    fireEvent.click(
      await screen.findByRole("button", { name: startButtonName }),
    );

    expect((await screen.findByRole("alert")).textContent).toContain(
      readOnlyMessage,
    );
    expect(onChanged).not.toHaveBeenCalled();
  });
});

const reopenButtonName = /^reabrir$/i;

// Backend simulado del detalle completo: el defecto Resuelto en el sprint Activo, las historias, los
// sprints y el cambio de estado, que reabre el defecto y borra el sprint de resolución.
function detailBackend() {
  const base = backend("resolved", (body) =>
    jsonResponse(200, defect(body.status as DefectStatus)),
  );
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, "http://localhost");
    if (init?.method === undefined || init.method === "GET") {
      if (url.pathname.endsWith(`/api/v1/defects/${defectId}`)) {
        return jsonResponse(200, defect("resolved", sprintRef(activeSprint)));
      }
      if (url.pathname.endsWith("/stories")) {
        return jsonResponse(200, {
          stories: [],
          appliedFilters: { status: [], priority: [] },
          emptyReason: "no_matches",
        });
      }
    }
    return base(input, init);
  };
}

describe("007/US5 - Reabrir un defecto resuelto", () => {
  let fetchMock: FetchMock;

  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(detailBackend());
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Escenario: el botón "Reabrir" aparece solo en un defecto Resuelto (US5, RN-15).
  it("el botón Reabrir aparece solo en un defecto Resuelto", async () => {
    renderActions(defect("resolved", sprintRef(activeSprint)));
    expect(
      await screen.findByRole("button", { name: reopenButtonName }),
    ).toBeTruthy();
    cleanup();

    for (const status of ["open", "in_progress"] as const) {
      renderActions(defect(status));
      await screen.findByRole("button");
      expect(
        screen.queryByRole("button", { name: reopenButtonName }),
      ).toBeNull();
      cleanup();
    }
  });

  // Escenario: al reabrir se envía status open y el detalle muestra el sprint de resolución vacío y
  // los campos de vínculo habilitados otra vez (US5, SC-009).
  it("después de reabrir, el detalle muestra el sprint de resolución vacío y habilita los vínculos", async () => {
    render(
      <SessionProvider>
        <DefectDetail defectId={defectId} />
      </SessionProvider>,
    );

    const resolutionTerm = await screen.findByText("Sprint de resolución");
    await waitFor(() =>
      expect(resolutionTerm.nextElementSibling?.textContent).toBe(
        activeSprint.name,
      ),
    );
    expect(
      screen.getByLabelText<HTMLSelectElement>(/^historia/i).disabled,
    ).toBe(true);
    expect(
      screen.getByLabelText<HTMLSelectElement>(/^sprint de detecci[oó]n/i)
        .disabled,
    ).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: reopenButtonName }));

    await waitFor(() =>
      expect(resolutionTerm.nextElementSibling?.textContent).toBe(
        "Sin resolver",
      ),
    );
    expect(statusRequests(fetchMock)[0]?.body).toEqual({ status: "open" });
    expect(
      screen.getByLabelText<HTMLSelectElement>(/^historia/i).disabled,
    ).toBe(false);
    expect(
      screen.getByLabelText<HTMLSelectElement>(/^sprint de detecci[oó]n/i)
        .disabled,
    ).toBe(false);
  });
});

const discardButtonName = /^descartar$/i;
const confirmDiscardName = /confirmar descarte/i;
const cancelButtonName = /^cancelar$/i;

describe("007/US6 - Descartar un defecto que no corresponde", () => {
  let fetchMock: FetchMock;

  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(backend("open"));
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Escenario: "Descartar" aparece en un defecto Abierto y en uno En progreso, y no en un Resuelto
  // ni en un Descartado (US6, RN-12).
  it("el botón Descartar aparece en Abierto y En progreso", async () => {
    for (const status of ["open", "in_progress"] as const) {
      renderActions(defect(status));
      expect(
        await screen.findByRole("button", { name: discardButtonName }),
      ).toBeTruthy();
      cleanup();
    }

    renderActions(defect("resolved", sprintRef(activeSprint)));
    await screen.findByRole("button", { name: reopenButtonName });
    expect(
      screen.queryByRole("button", { name: discardButtonName }),
    ).toBeNull();
    cleanup();

    renderActions(defect("discarded"));
    expect(screen.queryByRole("button")).toBeNull();
  });

  // Escenario: "Descartar" pide confirmación y no envía nada hasta confirmar (US6, FR-017).
  it("Descartar pide confirmación antes de enviar el cambio", async () => {
    const onChanged = renderActions(defect("open"));

    fireEvent.click(
      await screen.findByRole("button", { name: discardButtonName }),
    );
    expect(
      await screen.findByRole("button", { name: confirmDiscardName }),
    ).toBeTruthy();
    expect(statusRequests(fetchMock)).toHaveLength(0);

    fireEvent.click(screen.getByRole("button", { name: confirmDiscardName }));

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(statusRequests(fetchMock)).toHaveLength(1);
    expect(statusRequests(fetchMock)[0]?.url).toContain(statusPath);
    expect(statusRequests(fetchMock)[0]?.body).toEqual({
      status: "discarded",
    });
    expect(onChanged.mock.calls[0]?.[0].status).toBe("discarded");
  });

  // Escenario: cancelar la confirmación no envía nada y deja el defecto como estaba (US6).
  it("cancelar la confirmación no descarta el defecto", async () => {
    const onChanged = renderActions(defect("in_progress"));

    fireEvent.click(
      await screen.findByRole("button", { name: discardButtonName }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: cancelButtonName }),
    );

    expect(
      screen.queryByRole("button", { name: confirmDiscardName }),
    ).toBeNull();
    expect(
      screen.getByRole("button", { name: discardButtonName }),
    ).toBeTruthy();
    expect(statusRequests(fetchMock)).toHaveLength(0);
    expect(onChanged).not.toHaveBeenCalled();
  });
});
