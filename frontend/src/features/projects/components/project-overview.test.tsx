import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { ProjectOverview } from "@/features/projects/components/project-overview";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ProjectDetail, ProjectMemberView } from "@/types/project";

// Cobertura adicional de la pantalla de estado (T061), sin tarea asignada en tasks.md. Fija la
// deducción de isViewerOwner en ProjectOverview: la vista de estado no informa quién consulta, y
// la pantalla trata como propietario a quien recibe los correos de los integrantes, porque el
// backend solo los envía al propietario (FR-030). Con correos se ofrecen el formulario de alta y
// el botón Quitar; sin correos no se ofrece ninguna acción de administración. Solo se sustituye
// fetch, que es la frontera con el backend.

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

// Integrantes como los ve el propietario: Ana, que es la propietaria, y Bruno, ambos con correo.
const membersWithEmail: ProjectMemberView[] = [
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
const membersWithoutEmail: ProjectMemberView[] = membersWithEmail.map(
  (member) => ({
    userId: member.userId,
    fullName: member.fullName,
    isOwner: member.isOwner,
    joinedAt: member.joinedAt,
  }),
);

// Arma un proyecto En curso con los integrantes indicados.
function projectWith(members: ProjectMemberView[]): ProjectDetail {
  return {
    id: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
    name: "Portal de Clientes",
    description: null,
    status: "in_progress",
    startDate: "2026-10-01",
    plannedEndDate: "2026-12-20",
    actualEndDate: null,
    hoursPerStoryPoint: 6.5,
    readOnly: false,
    owner: {
      id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
    },
    members,
    hasActiveSprint: false,
    activeSprint: null,
    backlog: { countsByState: {}, estimatedStoryPoints: 0, estimatedHours: 0 },
  };
}

// Hace que fetch responda con el proyecto indicado y monta la pantalla dentro del proveedor de
// sesión; espera a que aparezca la lista de integrantes.
async function renderOverview(fetchMock: FetchMock, project: ProjectDetail) {
  fetchMock.mockImplementation(
    async () =>
      new Response(JSON.stringify(project), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
  );
  render(
    <SessionProvider>
      <ProjectOverview projectId={project.id} />
    </SessionProvider>,
  );
  await screen.findByRole("list", { name: /integrantes/i });
}

describe("Pantalla de estado - acciones de administración según los correos", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch sustituido.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
  });

  // Desmonta lo renderizado y restaura fetch para no contaminar otras pruebas.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("con correos en los integrantes muestra el formulario de alta y el botón Quitar", async () => {
    await renderOverview(fetchMock, projectWith(membersWithEmail));

    expect(
      screen.getByRole("button", { name: /agregar integrante/i }),
    ).toBeTruthy();
    expect(screen.getAllByRole("button", { name: /quitar/i })).toHaveLength(1);
  });

  it("sin correos en los integrantes no muestra ninguna acción de administración", async () => {
    await renderOverview(fetchMock, projectWith(membersWithoutEmail));

    expect(
      screen.queryByRole("button", { name: /agregar integrante/i }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: /quitar/i })).toBeNull();
  });
});

describe("003/RC-07 - Navegación de la vista del proyecto al backlog", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch sustituido.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
  });

  // Desmonta lo renderizado y restaura fetch para no contaminar otras pruebas.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Integración con la 002 (T076): la vista del proyecto ofrece un enlace que lleva a la pantalla
  // del backlog de ese mismo proyecto, tanto al propietario como a un integrante.
  it("La vista del proyecto ofrece un enlace al backlog", async () => {
    for (const members of [membersWithEmail, membersWithoutEmail]) {
      const project = projectWith(members);
      await renderOverview(fetchMock, project);

      const link = screen.getByRole("link", { name: /backlog/i });
      expect(link.getAttribute("href")).toBe(`/projects/${project.id}/backlog`);
      cleanup();
    }
  });
});
