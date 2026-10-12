import { act, cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getJson, isRecord, type ApiResult } from "@/lib/api-client";
import { SessionProvider } from "@/providers/session-provider";
import { writeToken } from "@/lib/session-storage";
import { useRequireSession } from "./use-session";

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador
// montado, y así la prueba puede observar a qué ruta lleva la guardia.
const router = vi.hoisted(() => ({
  push: vi.fn<(href: string) => void>(),
  replace: vi.fn<(href: string) => void>(),
  refresh: vi.fn<() => void>(),
  back: vi.fn<() => void>(),
  forward: vi.fn<() => void>(),
  prefetch: vi.fn<(href: string) => void>(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => router,
}));

// Token de una sesión emitida y respuesta del contrato para una sesión
// inválida (contracts/openapi.yaml, respuesta SesionInvalida).
const issuedToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const invalidSessionMessage = "Se requiere iniciar sesión.";

// Ruta protegida que se consulta en el escenario del 401.
const protectedPath = "/api/v1/users/me";

// Arma el doble de fetch que responde siempre con el estado y el cuerpo
// indicados; cada llamada recibe una respuesta nueva porque el cuerpo de una
// Response solo se puede leer una vez.
function stubFetch(status: number, body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(async () =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

// Acepta cualquier cuerpo con forma de objeto: el escenario solo observa el
// rechazo, no el contenido de una respuesta exitosa.
function isAnyRecord(value: unknown): value is Record<string, unknown> {
  return isRecord(value);
}

// Devuelve los valores guardados en sessionStorage, sin depender del nombre
// de la clave que elija la implementación (research.md, sección 9).
function storedValues(): (string | null)[] {
  return Array.from({ length: sessionStorage.length }, (_, i) =>
    sessionStorage.getItem(sessionStorage.key(i) ?? ""),
  );
}

// Rutas a las que la guardia pidió navegar, con push o con replace, sin la
// cadena de consulta: el aviso de volver a iniciar sesión puede viajar en ella.
function navigatedPaths(): string[] {
  return [...router.push.mock.calls, ...router.replace.mock.calls].map(
    ([href]) => href.split("?")[0],
  );
}

// Pantalla protegida de prueba: solo muestra su contenido si la guardia
// devuelve una sesión.
function ProtectedScreen() {
  const token = useRequireSession();
  return token ? <p>Contenido protegido</p> : null;
}

// Monta la pantalla protegida dentro del proveedor de sesión, como en el
// layout raíz.
function renderProtectedScreen() {
  return render(
    <SessionProvider>
      <ProtectedScreen />
    </SessionProvider>,
  );
}

describe("001/US3 - Protección de las funciones del sistema", () => {
  // Cada escenario parte de una pestaña sin sesión y sin navegaciones previas.
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Sin token guardado la guardia trata a la persona como visitante: no
  // muestra la pantalla protegida y la lleva al inicio de sesión (FR-019).
  it("Caso de error — acceso a una función protegida sin sesión", async () => {
    stubFetch(401, { error: invalidSessionMessage });

    const view = renderProtectedScreen();

    await waitFor(() => expect(navigatedPaths()).toContain("/login"));
    expect(view.queryByText("Contenido protegido")).toBeNull();
  });

  // Con una sesión que la pestaña todavía cree vigente, la API responde 401
  // porque venció entre una acción y la siguiente: el cliente HTTP limpia el
  // token y la guardia lleva al inicio de sesión (FR-015, FR-016).
  it("Caso límite — la sesión vence entre una acción y la siguiente", async () => {
    writeToken(issuedToken);
    stubFetch(401, { error: invalidSessionMessage });

    const view = renderProtectedScreen();
    expect(view.getByText("Contenido protegido")).toBeDefined();
    expect(navigatedPaths()).toHaveLength(0);

    let result: ApiResult<Record<string, unknown>> | undefined;
    await act(async () => {
      result = await getJson(protectedPath, isAnyRecord);
    });

    expect(result).toEqual({
      ok: false,
      error: { status: 401, message: invalidSessionMessage, fields: [] },
    });
    await waitFor(() => expect(navigatedPaths()).toContain("/login"));
    expect(storedValues()).not.toContain(issuedToken);
  });
});
