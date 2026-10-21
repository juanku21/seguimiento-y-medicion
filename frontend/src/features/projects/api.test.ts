import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { API_BASE_URL } from "@/config/env";
import {
  addProjectMember,
  changeProjectStatus,
  getProjectDetail,
  removeProjectMember,
  updateProject,
} from "@/features/projects/api";
import { writeToken } from "@/lib/session-storage";
import type { ProjectInput } from "@/types/project";

// Prueba transversal de seguridad (T087): identificadores interpolados sin codificar en las rutas de
// los clientes de API, hallazgo detectado en 003/US2. Cada función del cliente de proyectos que
// interpola un identificador lo recibe con "../" y "/", y la URL pedida a fetch debe conservar la
// ruta esperada con ese segmento codificado. createProject y listProjects no interpolan
// identificadores y quedan fuera. Solo se sustituye fetch; el cliente HTTP es el real.

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";
const maliciousID = "../../auth/me/x";
const encodedID = encodeURIComponent(maliciousID);
const otherMaliciousID = "../members/../../y";
const otherEncodedID = encodeURIComponent(otherMaliciousID);

const projectInput: ProjectInput = {
  name: "Proyecto",
  startDate: "2026-10-01",
  plannedEndDate: "2026-12-01",
  hoursPerStoryPoint: 4,
};

// Hace que fetch responda siempre con un 404 de ErrorResponse: la prueba observa solo la URL pedida,
// no el manejo de la respuesta.
function respondNotFound(fetchMock: FetchMock) {
  fetchMock.mockImplementation(
    async () =>
      new Response(JSON.stringify({ error: "No encontrado." }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
  );
}

// Devuelve la URL de la única llamada a fetch, para compararla con la ruta esperada.
function requestedURL(fetchMock: FetchMock): string {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const [input] = fetchMock.mock.calls[0];
  return input instanceof Request
    ? input.url
    : input instanceof URL
      ? input.href
      : input;
}

describe("Clientes de API de proyectos - codificación de identificadores en la ruta (T087)", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con sesión en la pestaña y un fetch sustituido que responde 404.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    respondNotFound(fetchMock);
    vi.stubGlobal("fetch", fetchMock);
  });

  // Restaura fetch y limpia la sesión para no contaminar otras pruebas.
  afterEach(() => {
    vi.unstubAllGlobals();
    sessionStorage.clear();
  });

  // updateProject interpola el projectId en PUT /api/v1/projects/{projectId}.
  it("updateProject codifica el identificador del proyecto", async () => {
    await updateProject(maliciousID, projectInput);
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}`,
    );
  });

  // addProjectMember interpola el projectId en POST /api/v1/projects/{projectId}/members.
  it("addProjectMember codifica el identificador del proyecto", async () => {
    await addProjectMember(maliciousID, "bruno@example.com");
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}/members`,
    );
  });

  // removeProjectMember interpola el projectId y el userId en
  // DELETE /api/v1/projects/{projectId}/members/{userId}; ambos segmentos se codifican.
  it("removeProjectMember codifica los identificadores del proyecto y del integrante", async () => {
    await removeProjectMember(maliciousID, otherMaliciousID);
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}/members/${otherEncodedID}`,
    );
  });

  // getProjectDetail interpola el projectId en GET /api/v1/projects/{projectId}.
  it("getProjectDetail codifica el identificador del proyecto", async () => {
    await getProjectDetail(maliciousID);
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}`,
    );
  });

  // changeProjectStatus interpola el projectId en PATCH /api/v1/projects/{projectId}/status.
  it("changeProjectStatus codifica el identificador del proyecto", async () => {
    await changeProjectStatus(maliciousID, "in_progress");
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}/status`,
    );
  });
});
