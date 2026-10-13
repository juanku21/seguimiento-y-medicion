import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useSession } from "@/hooks/use-session";
import { wasSessionRejected, writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import { LoginForm } from "./login-form";
import { LogoutButton } from "./logout-button";
import { ProfileCard } from "./profile-card";

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
  useSearchParams: () => new URLSearchParams(""),
}));

// Tokens de las sesiones de los escenarios, rutas del contrato
// (contracts/openapi.yaml) y perfil del titular.
const issuedToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const newToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEiLCJuIjoyfQ.otra";
const invalidSessionMessage = "Se requiere iniciar sesión.";
const logoutPath = "/api/v1/auth/logout";
const loginPath = "/api/v1/auth/login";
const profilePath = "/api/v1/users/me";
const validPassword = "Proyecto2026";
const ownProfile = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
  email: "ana@example.com",
};

// Respuesta simulada de una ruta: estado y cuerpo; sin cuerpo para el 204.
type StubResponse = { status: number; body?: unknown };

// Petición observada por el doble de fetch, normalizada a URL, método,
// cabecera Authorization y cuerpo en texto.
type SentRequest = {
  url: string;
  method: string;
  authorization: string | null;
  body: string | null;
};

// Normaliza los argumentos de fetch, tanto si el cliente HTTP lo llamó con una
// URL y opciones como con un Request.
async function toSentRequest(
  input: Parameters<typeof fetch>[0],
  init: Parameters<typeof fetch>[1],
): Promise<SentRequest> {
  if (input instanceof Request) {
    const text = await input.clone().text();
    return {
      url: input.url,
      method: input.method,
      authorization: input.headers.get("Authorization"),
      body: text === "" ? null : text,
    };
  }
  return {
    url: input instanceof URL ? input.href : input,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
    body: init?.body == null ? null : String(init.body),
  };
}

// Arma el doble de fetch que responde según la ruta pedida y registra cada
// petición. Cada llamada recibe una respuesta nueva porque el cuerpo de una
// Response solo se puede leer una vez; el 204 se arma sin cuerpo, como exige
// la especificación de fetch.
function stubRoutes(routes: Record<string, StubResponse>) {
  const sent: SentRequest[] = [];
  const fetchMock = vi.fn<typeof fetch>(async (input, init) => {
    const request = await toSentRequest(input, init);
    sent.push(request);
    const path = Object.keys(routes).find((p) => request.url.endsWith(p));
    if (path === undefined) {
      return new Response(null, { status: 404 });
    }
    const { status, body } = routes[path];
    return body === undefined
      ? new Response(null, { status })
      : new Response(JSON.stringify(body), {
          status,
          headers: { "Content-Type": "application/json" },
        });
  });
  vi.stubGlobal("fetch", fetchMock);
  return sent;
}

// Devuelve los valores guardados en sessionStorage, sin depender del nombre
// de la clave que elija la implementación (research.md, sección 9).
function storedValues(): (string | null)[] {
  return Array.from({ length: sessionStorage.length }, (_, i) =>
    sessionStorage.getItem(sessionStorage.key(i) ?? ""),
  );
}

// Rutas completas a las que se pidió navegar, con push o con replace.
function navigations(): string[] {
  return [...router.push.mock.calls, ...router.replace.mock.calls].map(
    ([href]) => href,
  );
}

// Muestra el estado de sesión que expone el proveedor al resto de la
// interfaz: "visitante" sin token y "con sesión" con un token vigente.
function SessionProbe() {
  const { token } = useSession();
  return (
    <p data-testid="session-state">
      {token === null ? "visitante" : "con sesión"}
    </p>
  );
}

// Devuelve el estado de sesión que muestra la sonda.
function sessionState(): string | null {
  return screen.getByTestId("session-state").textContent;
}

// Pulsa el botón de cierre de sesión.
function clickLogout() {
  fireEvent.click(screen.getByRole("button", { name: /cerrar sesión/i }));
}

describe("001/US5 - Cierre de sesión", () => {
  // Cada escenario parte de una pestaña sin sesión y sin dobles previos.
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Con sesión vigente, el botón pide el cierre a la API; ante el 204 la
  // pestaña descarta el token y la interfaz vuelve al estado de visitante, sin
  // la marca de sesión rechazada: el cierre es voluntario, no un rechazo.
  it("Caso normal — usuario con sesión vigente cierra su sesión", async () => {
    writeToken(issuedToken);
    const sent = stubRoutes({ [logoutPath]: { status: 204 } });

    render(
      <SessionProvider>
        <LogoutButton />
        <SessionProbe />
      </SessionProvider>,
    );
    expect(sessionState()).toBe("con sesión");
    clickLogout();

    await waitFor(() => expect(sessionState()).toBe("visitante"));
    expect(sent.some((r) => r.url.endsWith(logoutPath))).toBe(true);
    expect(storedValues()).not.toContain(issuedToken);
    expect(wasSessionRejected()).toBe(false);
  });

  // Tras cerrar la sesión, un nuevo inicio con credenciales correctas guarda
  // el token de la sesión nueva, distinto del cerrado, y la interfaz vuelve a
  // tener sesión vigente.
  it("Caso alternativo — usuario que cerró su sesión vuelve a iniciar sesión", async () => {
    writeToken(issuedToken);
    stubRoutes({
      [logoutPath]: { status: 204 },
      [loginPath]: {
        status: 200,
        body: { token: newToken, expiresAt: "2026-10-12T20:00:00Z" },
      },
    });

    render(
      <SessionProvider>
        <LogoutButton />
        <LoginForm />
        <SessionProbe />
      </SessionProvider>,
    );
    clickLogout();
    await waitFor(() => expect(sessionState()).toBe("visitante"));

    fireEvent.change(screen.getByLabelText(/correo electrónico/i), {
      target: { value: ownProfile.email },
    });
    fireEvent.change(screen.getByLabelText(/contraseña/i), {
      target: { value: validPassword },
    });
    fireEvent.click(screen.getByRole("button", { name: /iniciar sesión/i }));

    await waitFor(() => expect(sessionState()).toBe("con sesión"));
    expect(storedValues()).toContain(newToken);
    expect(storedValues()).not.toContain(issuedToken);
  });

  // Después del cierre, la función protegida vuelve a exigir credenciales: la
  // guardia lleva a /login sin el aviso de sesión rechazada, y ninguna
  // petición posterior al cierre viaja con el token cerrado.
  it("Caso límite — la sesión cerrada no da acceso a funciones protegidas", async () => {
    writeToken(issuedToken);
    const sent = stubRoutes({
      [profilePath]: { status: 200, body: ownProfile },
      [logoutPath]: { status: 204 },
    });

    render(
      <SessionProvider>
        <LogoutButton />
        <ProfileCard />
        <SessionProbe />
      </SessionProvider>,
    );
    expect(await screen.findByText(ownProfile.fullName)).toBeDefined();
    clickLogout();

    await waitFor(() => expect(navigations()).toContain("/login"));
    expect(navigations()).not.toContain("/login?motivo=sesion");
    const logoutIndex = sent.findIndex((r) => r.url.endsWith(logoutPath));
    expect(logoutIndex).toBeGreaterThanOrEqual(0);
    const afterLogout = sent.slice(logoutIndex + 1);
    expect(
      afterLogout.some((r) => r.authorization === `Bearer ${issuedToken}`),
    ).toBe(false);
  });

  // El cierre afecta solo a la sesión de esta pestaña: se envía un POST al
  // endpoint de cierre con el token propio en la cabecera y sin ningún
  // identificador de cuenta ni de otras sesiones en el cuerpo (FR-017).
  it("Caso límite — cerrar en un dispositivo no afecta la sesión del otro", async () => {
    writeToken(issuedToken);
    const sent = stubRoutes({ [logoutPath]: { status: 204 } });

    render(
      <SessionProvider>
        <LogoutButton />
        <SessionProbe />
      </SessionProvider>,
    );
    clickLogout();

    await waitFor(() => expect(sessionState()).toBe("visitante"));
    const logoutRequests = sent.filter((r) => r.url.endsWith(logoutPath));
    expect(logoutRequests).toHaveLength(1);
    expect(logoutRequests[0].method).toBe("POST");
    expect(logoutRequests[0].authorization).toBe(`Bearer ${issuedToken}`);
    expect(logoutRequests[0].body).toBeNull();
  });

  // La API rechaza el cierre con 401 porque la sesión de la pestaña ya no es
  // válida: la interfaz queda como visitante, descarta el token y no muestra
  // datos de ninguna cuenta.
  it("Caso de error — visitante sin sesión solicita cerrar sesión", async () => {
    writeToken(issuedToken);
    const sent = stubRoutes({
      [logoutPath]: { status: 401, body: { error: invalidSessionMessage } },
    });

    const view = render(
      <SessionProvider>
        <LogoutButton />
        <SessionProbe />
      </SessionProvider>,
    );
    clickLogout();

    await waitFor(() => expect(sessionState()).toBe("visitante"));
    expect(sent.some((r) => r.url.endsWith(logoutPath))).toBe(true);
    expect(storedValues()).not.toContain(issuedToken);
    const shown = view.container.textContent ?? "";
    expect(shown).not.toContain(ownProfile.fullName);
    expect(shown).not.toContain(ownProfile.email);
  });
});
