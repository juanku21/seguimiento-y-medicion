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
  createStory,
  deleteStory,
  estimateStory,
  getStory,
  listStories,
  updateStory,
} from "@/features/backlog/api";
import { writeToken } from "@/lib/session-storage";

// Prueba transversal de seguridad (T087): identificadores interpolados sin codificar en las rutas de
// los clientes de API, hallazgo detectado en 003/US2. Cada función del cliente del backlog recibe un
// identificador con "../" y "/", y la URL pedida a fetch debe conservar la ruta esperada con ese
// segmento codificado, de modo que el identificador no pueda subir de nivel ni apuntar a otro
// endpoint. Solo se sustituye fetch, que es la frontera con el backend; el cliente HTTP es el real.

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";
const maliciousID = "../../auth/me/x";
const encodedID = encodeURIComponent(maliciousID);

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

describe("Clientes de API del backlog - codificación de identificadores en la ruta (T087)", () => {
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

  // createStory interpola el projectId en POST /api/v1/projects/{projectId}/stories.
  it("createStory codifica el identificador del proyecto", async () => {
    await createStory(maliciousID, { title: "Historia", priority: "high" });
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}/stories`,
    );
  });

  // getStory interpola el storyId en GET /api/v1/stories/{storyId}.
  it("getStory codifica el identificador de la historia", async () => {
    await getStory(maliciousID);
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/stories/${encodedID}`,
    );
  });

  // estimateStory interpola el storyId en PUT /api/v1/stories/{storyId}/story-points.
  it("estimateStory codifica el identificador de la historia", async () => {
    await estimateStory(maliciousID, 5);
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/stories/${encodedID}/story-points`,
    );
  });

  // updateStory interpola el storyId en PATCH /api/v1/stories/{storyId}.
  it("updateStory codifica el identificador de la historia", async () => {
    await updateStory(maliciousID, { title: "Nuevo título" });
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/stories/${encodedID}`,
    );
  });

  // deleteStory interpola el storyId en DELETE /api/v1/stories/{storyId}.
  it("deleteStory codifica el identificador de la historia", async () => {
    await deleteStory(maliciousID);
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/stories/${encodedID}`,
    );
  });

  // listStories interpola el projectId en GET /api/v1/projects/{projectId}/stories, con y sin
  // consulta de filtros.
  it("listStories codifica el identificador del proyecto", async () => {
    await listStories(maliciousID, { status: [], priority: [] });
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}/stories`,
    );
  });

  it("listStories codifica el identificador del proyecto con filtros", async () => {
    await listStories(maliciousID, { status: ["pending"], priority: ["high"] });
    expect(requestedURL(fetchMock)).toBe(
      `${API_BASE_URL}/api/v1/projects/${encodedID}/stories?status=pending&priority=high`,
    );
  });
});
