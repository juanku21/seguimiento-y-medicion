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
  within,
} from "@testing-library/react";
import { ProjectOverview } from "@/features/projects/components/project-overview";
import { StatusActions } from "@/features/projects/components/status-actions";
import { isRecord } from "@/lib/api-client";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  ProjectDetail,
  ProjectMemberView,
  ProjectStatus,
} from "@/types/project";

// Pruebas del avance de estado del proyecto (T073). Atraviesan los componentes, la llamada
// changeProjectStatus y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el
// backend. Las reglas de la máquina de estados las resuelve el backend (T074 y T075): acá se
// verifica qué ofrece la interfaz y cómo refleja la respuesta.
//
// Contrato que fijan estas pruebas:
// - StatusActions recibe project (ProjectDetail) y onStatusChange(project). Ofrece **solo** la
//   transición disponible según el estado actual: con Planificado, un único botón "Pasar a En
//   curso"; con En curso, un único botón "Finalizar proyecto"; con Finalizado, ningún botón.
// - Al pulsarlo, changeProjectStatus(projectId, status) envía PATCH
//   /api/v1/projects/{projectId}/status con el cuerpo { status } y la sesión de la pestaña. Si el
//   backend acepta, entrega el ProjectDetail recibido a onStatusChange; si rechaza (403, 409),
//   muestra el mensaje del backend con role="alert" y no llama a onStatusChange.
// - La pantalla del proyecto (ProjectOverview, que monta page.tsx) incluye StatusActions solo para
//   el propietario, actualiza la tarjeta de estado con el proyecto recibido y, cuando el proyecto
//   es de solo lectura (readOnly), oculta todas las acciones de administración: el cambio de
//   estado, el formulario de alta, el botón Quitar y el enlace de edición (FR-018, US4-2).

// Sustituye el enrutador de Next.js que usa la guardia de sesión: fuera del App Router no hay
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
  useSearchParams: () => new URLSearchParams(""),
}));

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";

// Mensajes de rechazo del contrato (409 y 403 de PATCH /status).
const sameStatusMessage = "El proyecto ya está En curso.";
const activeSprintMessage =
  "No se puede finalizar el proyecto mientras tenga un sprint activo. Cerralo primero.";
const readOnlyMessage = "El proyecto está finalizado y es de solo lectura.";
const notOwnerMessage =
  "Solo el propietario del proyecto puede realizar esta acción.";

// Integrantes como los ve la propietaria: Ana y Bruno, ambos con correo (FR-030).
const ownerViewMembers: ProjectMemberView[] = [
  {
    userId: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
    fullName: "Ana Pérez",
    email: "ana@example.com",
    isOwner: true,
    joinedAt: "2026-10-01T12:00:00Z",
  },
  {
    userId: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
    fullName: "Bruno Gómez",
    email: "bruno@example.com",
    isOwner: false,
    joinedAt: "2026-10-03T09:15:00Z",
  },
];

// Los mismos integrantes como los ve quien no es propietario: el backend omite los correos.
const memberViewMembers: ProjectMemberView[] = ownerViewMembers.map(
  (member) => ({
    userId: member.userId,
    fullName: member.fullName,
    isOwner: member.isOwner,
    joinedAt: member.joinedAt,
  }),
);

// Proyecto Planificado de Ana, visto por ella, sin backlog ni sprints.
const plannedProject: ProjectDetail = {
  id: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
  name: "Portal de Clientes",
  description: null,
  status: "planned",
  startDate: "2026-10-01",
  plannedEndDate: "2026-12-20",
  actualEndDate: null,
  hoursPerStoryPoint: 6.5,
  readOnly: false,
  owner: {
    id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
    fullName: "Ana Pérez",
  },
  members: ownerViewMembers,
  hasActiveSprint: false,
  activeSprint: null,
  backlog: { countsByState: {}, estimatedStoryPoints: 0, estimatedHours: 0 },
};

// El mismo proyecto En curso.
const inProgressProject: ProjectDetail = {
  ...plannedProject,
  status: "in_progress",
};

// Arma el proyecto Finalizado con la fecha real indicada; queda de solo lectura (FR-018).
function finishedOn(actualEndDate: string): ProjectDetail {
  return {
    ...plannedProject,
    status: "finished",
    actualEndDate,
    readOnly: true,
  };
}

// Arma una respuesta JSON con el estado y el cuerpo indicados.
function jsonResponse(
  status: number,
  body: ProjectDetail | ErrorResponse,
): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Lee el estado destino del cuerpo de un PATCH, o null si no lo trae.
function targetStatus(body: RequestInit["body"]): ProjectStatus | null {
  if (typeof body !== "string") {
    return null;
  }
  const parsed: unknown = JSON.parse(body);
  if (!isRecord(parsed)) {
    return null;
  }
  const { status } = parsed;
  return status === "planned" ||
    status === "in_progress" ||
    status === "finished"
    ? status
    : null;
}

// Backend falso para la pantalla: el GET devuelve el proyecto vigente y el PATCH lo reemplaza por
// el que corresponde al estado destino; un destino no previsto se rechaza con 409.
function fakeBackend(
  fetchMock: FetchMock,
  initial: ProjectDetail,
  afterChange: Partial<Record<ProjectStatus, ProjectDetail>>,
) {
  let current = initial;
  fetchMock.mockImplementation(async (_input, init) => {
    if (init?.method === "PATCH") {
      const target = targetStatus(init.body);
      const next = target === null ? undefined : afterChange[target];
      if (next === undefined) {
        return jsonResponse(409, { error: "Transición no prevista." });
      }
      current = next;
    }
    return jsonResponse(200, current);
  });
}

// Extrae la URL, el método, la cabecera Authorization y el estado destino de cada PATCH enviado.
function patchRequests(fetchMock: FetchMock) {
  return fetchMock.mock.calls
    .filter(([, init]) => init?.method === "PATCH")
    .map(([input, init]) => ({
      url: input instanceof URL ? input.href : String(input),
      authorization: new Headers(init?.headers).get("Authorization"),
      status: targetStatus(init?.body),
    }));
}

// Monta la pantalla del proyecto dentro del proveedor de sesión y espera a que cargue.
async function renderOverview(projectId: string) {
  render(
    <SessionProvider>
      <ProjectOverview projectId={projectId} />
    </SessionProvider>,
  );
  await screen.findByRole("list", { name: /integrantes/i });
}

// Monta solo las acciones de estado y devuelve la función que recibe el proyecto actualizado.
function renderActions(project: ProjectDetail) {
  const onStatusChange = vi.fn<(project: ProjectDetail) => void>();
  render(
    <SessionProvider>
      <StatusActions project={project} onStatusChange={onStatusChange} />
    </SessionProvider>,
  );
  return onStatusChange;
}

// Tarjeta de estado de la pantalla.
function statusCard(): HTMLElement {
  return screen.getByRole("region", { name: "Estado del proyecto" });
}

// Texto de la fecha de finalización real que muestra la tarjeta, o null si no la muestra.
function actualEndDateText(): string | null {
  const label = within(statusCard()).queryByText("Finalización real");
  return label?.nextElementSibling?.textContent ?? null;
}

// Indica si el texto muestra la fecha AAAA-MM-DD en ese formato o como DD/MM/AAAA.
function showsDate(text: string | null, isoDate: string): boolean {
  const [year, month, day] = isoDate.split("-");
  return (
    text !== null &&
    (text.includes(isoDate) || text.includes(`${day}/${month}/${year}`))
  );
}

// Botones de cambio de estado que ofrece la interfaz.
function statusButtons(): HTMLElement[] {
  return screen.queryAllByRole("button", {
    name: /pasar a|finalizar|planificado/i,
  });
}

// Comprueba que la pantalla no ofrece ninguna acción de administración (FR-018).
function expectNoAdminActions() {
  expect(statusButtons()).toHaveLength(0);
  expect(
    screen.queryByRole("button", { name: /agregar integrante/i }),
  ).toBeNull();
  expect(screen.queryByRole("button", { name: /quitar/i })).toBeNull();
  expect(document.querySelector('a[href$="/edit"]')).toBeNull();
}

describe("002/US6 - Avanzar el estado del proyecto", () => {
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

  // Caso normal: la propietaria pasa el proyecto a En curso y después a Finalizado; ambas
  // transiciones se aceptan, la tarjeta refleja cada estado y al finalizar muestra la fecha real
  // y deja de ofrecer acciones de administración.
  it("Recorrido completo", async () => {
    const finished = finishedOn("2026-12-18");
    fakeBackend(fetchMock, plannedProject, {
      in_progress: inProgressProject,
      finished,
    });
    await renderOverview(plannedProject.id);

    fireEvent.click(screen.getByRole("button", { name: /pasar a en curso/i }));
    const finishButton = await screen.findByRole("button", {
      name: /finalizar proyecto/i,
    });
    expect(within(statusCard()).getByText("En curso")).toBeTruthy();

    fireEvent.click(finishButton);
    await within(statusCard()).findByText("Finalizado");
    expect(showsDate(actualEndDateText(), "2026-12-18")).toBe(true);
    expectNoAdminActions();

    const requests = patchRequests(fetchMock);
    expect(requests.map((request) => request.status)).toEqual([
      "in_progress",
      "finished",
    ]);
    for (const request of requests) {
      expect(
        request.url.endsWith(`/api/v1/projects/${plannedProject.id}/status`),
      ).toBe(true);
      expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    }
  });

  // Caso alternativo: finalizar antes de la fecha prevista se acepta; la tarjeta muestra la fecha
  // real anterior junto a la prevista, sin presentarlo como error.
  it("Finalización anticipada", async () => {
    fakeBackend(fetchMock, inProgressProject, {
      finished: finishedOn("2026-11-30"),
    });
    await renderOverview(inProgressProject.id);

    fireEvent.click(
      screen.getByRole("button", { name: /finalizar proyecto/i }),
    );
    await within(statusCard()).findByText("Finalizado");

    expect(showsDate(actualEndDateText(), "2026-11-30")).toBe(true);
    expect(
      showsDate(statusCard().textContent ?? "", plannedProject.plannedEndDate),
    ).toBe(true);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: un proyecto Finalizado se ve completo, con su fecha real, pero la interfaz
  // oculta todas las acciones de administración, tanto para la propietaria como para cualquier
  // otro integrante (FR-018, US4-2).
  it("Finalizado solo se consulta", async () => {
    const finished = finishedOn("2026-12-18");
    const viewers = [
      { name: "propietaria", members: ownerViewMembers },
      { name: "integrante", members: memberViewMembers },
    ];

    for (const viewer of viewers) {
      fakeBackend(fetchMock, { ...finished, members: viewer.members }, {});
      await renderOverview(finished.id);

      expect(within(statusCard()).getByText("Finalizado")).toBeTruthy();
      expect(showsDate(actualEndDateText(), "2026-12-18")).toBe(true);
      expect(screen.getAllByRole("listitem").length).toBeGreaterThan(0);
      expectNoAdminActions();
      cleanup();
    }
  });

  // Caso límite: finalizar el mismo día en que comenzó se acepta y la fecha real es igual a la
  // fecha de inicio.
  it("Finaliza el mismo día que empezó", async () => {
    fakeBackend(fetchMock, inProgressProject, {
      finished: finishedOn(plannedProject.startDate),
    });
    await renderOverview(inProgressProject.id);

    fireEvent.click(
      screen.getByRole("button", { name: /finalizar proyecto/i }),
    );
    await within(statusCard()).findByText("Finalizado");

    expect(showsDate(actualEndDateText(), plannedProject.startDate)).toBe(true);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: con un sprint activo y backlog cargado, el arranque se ofrece habilitado y se
  // envía directamente, sin exigir condiciones sobre el backlog ni los sprints.
  it("Arranque sin condiciones", async () => {
    const loadedPlanned: ProjectDetail = {
      ...plannedProject,
      hasActiveSprint: true,
      activeSprint: {
        id: "e1c4a7b9-2d58-4036-9f1e-7b5c8a2d4e03",
        name: "Sprint 1",
      },
      backlog: {
        countsByState: { pending: 4 },
        estimatedStoryPoints: 13,
        estimatedHours: 84.5,
      },
    };
    fakeBackend(fetchMock, loadedPlanned, {
      in_progress: { ...loadedPlanned, status: "in_progress" },
    });
    await renderOverview(loadedPlanned.id);

    const startButton = screen.getByRole("button", {
      name: /pasar a en curso/i,
    });
    expect(startButton.hasAttribute("disabled")).toBe(false);
    fireEvent.click(startButton);

    await screen.findByRole("button", { name: /finalizar proyecto/i });
    expect(within(statusCard()).getByText("En curso")).toBeTruthy();
    expect(patchRequests(fetchMock).map((request) => request.status)).toEqual([
      "in_progress",
    ]);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso de error: con el proyecto En curso, la interfaz solo ofrece finalizarlo; no hay ninguna
  // acción para volver a Planificado.
  it("Retroceso rechazado", () => {
    renderActions(inProgressProject);

    const buttons = statusButtons();
    expect(buttons).toHaveLength(1);
    expect(buttons[0].textContent).toMatch(/finalizar proyecto/i);
    expect(screen.queryByRole("button", { name: /planificado/i })).toBeNull();
  });

  // Caso de error: con el proyecto Planificado, la interfaz solo ofrece pasarlo a En curso; no hay
  // ninguna acción para finalizarlo directamente.
  it("Salto de estado rechazado", () => {
    renderActions(plannedProject);

    const buttons = statusButtons();
    expect(buttons).toHaveLength(1);
    expect(buttons[0].textContent).toMatch(/pasar a en curso/i);
    expect(screen.queryByRole("button", { name: /finalizar/i })).toBeNull();
  });

  // Caso de error: con el proyecto En curso no se ofrece pasarlo a En curso otra vez; si la vista
  // quedó vieja y el backend rechaza el mismo estado con 409, se muestra el motivo y nada cambia.
  it("Mismo estado rechazado", async () => {
    renderActions(inProgressProject);
    expect(
      screen.queryByRole("button", { name: /pasar a en curso/i }),
    ).toBeNull();
    cleanup();

    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: sameStatusMessage }),
    );
    const onStatusChange = renderActions(plannedProject);
    fireEvent.click(screen.getByRole("button", { name: /pasar a en curso/i }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(sameStatusMessage);
    expect(onStatusChange).not.toHaveBeenCalled();
    expect(patchRequests(fetchMock).map((request) => request.status)).toEqual([
      "in_progress",
    ]);
  });

  // Caso de error: con el proyecto Finalizado no se ofrece ningún cambio de estado; si la vista
  // quedó vieja y el backend lo rechaza por solo lectura, se muestra el motivo y nada cambia.
  it("Finalizado es solo lectura", async () => {
    renderActions(finishedOn("2026-12-18"));
    expect(statusButtons()).toHaveLength(0);
    cleanup();

    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: readOnlyMessage }),
    );
    const onStatusChange = renderActions(inProgressProject);
    fireEvent.click(
      screen.getByRole("button", { name: /finalizar proyecto/i }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(readOnlyMessage);
    expect(onStatusChange).not.toHaveBeenCalled();
  });

  // Caso de error: quien es integrante pero no propietario ve el proyecto sin ninguna acción de
  // cambio de estado.
  it("No es propietario", async () => {
    fakeBackend(
      fetchMock,
      { ...inProgressProject, members: memberViewMembers },
      {},
    );
    await renderOverview(inProgressProject.id);

    expect(within(statusCard()).getByText("En curso")).toBeTruthy();
    expect(statusButtons()).toHaveLength(0);
  });

  // Caso de error: si el backend rechaza el cambio por falta de permiso (403), se muestra el
  // motivo y el proyecto no cambia.
  it("No es propietario: muestra el rechazo del backend", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(403, { error: notOwnerMessage }),
    );
    const onStatusChange = renderActions(plannedProject);
    fireEvent.click(screen.getByRole("button", { name: /pasar a en curso/i }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(notOwnerMessage);
    expect(onStatusChange).not.toHaveBeenCalled();
  });

  // Caso de error: con un sprint activo, el backend rechaza la finalización con 409; la pantalla
  // muestra que primero hay que cerrar el sprint y el proyecto sigue En curso, sin fecha real.
  it("Finalizar con sprint activo", async () => {
    const withActiveSprint: ProjectDetail = {
      ...inProgressProject,
      hasActiveSprint: true,
      activeSprint: {
        id: "e1c4a7b9-2d58-4036-9f1e-7b5c8a2d4e03",
        name: "Sprint 4",
      },
    };
    fetchMock.mockImplementation(async (_input, init) =>
      init?.method === "PATCH"
        ? jsonResponse(409, { error: activeSprintMessage })
        : jsonResponse(200, withActiveSprint),
    );
    await renderOverview(withActiveSprint.id);

    fireEvent.click(
      screen.getByRole("button", { name: /finalizar proyecto/i }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(activeSprintMessage);
    expect(within(statusCard()).getByText("En curso")).toBeTruthy();
    expect(actualEndDateText()).toBeNull();
    expect(
      screen.getByRole("button", { name: /finalizar proyecto/i }),
    ).toBeTruthy();
  });
});
