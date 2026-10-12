import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SessionProvider } from "@/providers/session-provider";
import { LoginForm } from "./login-form";

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador
// montado, y así la prueba puede observar a qué ruta navega el formulario.
const router = vi.hoisted(() => ({
  push: vi.fn<(href: string) => void>(),
  replace: vi.fn<(href: string) => void>(),
  refresh: vi.fn<() => void>(),
  back: vi.fn<() => void>(),
  forward: vi.fn<() => void>(),
  prefetch: vi.fn<(href: string) => void>(),
}));

// Cadena de consulta de la URL actual de /login. Cada escenario la fija antes
// de montar el formulario; por defecto está vacía, como al entrar sin motivo.
const currentSearch = vi.hoisted(() => ({ value: "" }));

vi.mock("next/navigation", () => ({
  useRouter: () => router,
  useSearchParams: () => new URLSearchParams(currentSearch.value),
}));

// Datos de la cuenta de los escenarios y respuestas del contrato
// POST /api/v1/auth/login (contracts/openapi.yaml).
const validPassword = "Proyecto2026";
const issuedToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const genericError = "Correo electrónico o contraseña incorrectos.";

type LoginBody = { email: string; password: string };

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

// Extrae URL, método y cuerpo de la petición que envió el formulario, tanto
// si el cliente HTTP llamó a fetch con una URL y opciones como con un Request.
async function sentRequest(fetchMock: ReturnType<typeof stubFetch>) {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const [input, init] = fetchMock.mock.calls[0];
  if (input instanceof Request) {
    return {
      url: input.url,
      method: input.method,
      body: JSON.parse(await input.clone().text()) as LoginBody,
    };
  }
  return {
    url: input instanceof URL ? input.href : input,
    method: init?.method ?? "GET",
    body: JSON.parse(String(init?.body)) as LoginBody,
  };
}

// Devuelve los valores guardados en sessionStorage, sin depender del nombre
// de la clave que elija la implementación (research.md, sección 9).
function storedValues(): (string | null)[] {
  return Array.from({ length: sessionStorage.length }, (_, i) =>
    sessionStorage.getItem(sessionStorage.key(i) ?? ""),
  );
}

// Rutas a las que el formulario pidió navegar, con push o con replace.
function navigations(): string[] {
  return [...router.push.mock.calls, ...router.replace.mock.calls].map(
    ([href]) => href,
  );
}

// Monta el formulario dentro del proveedor de sesión, como en el layout raíz,
// completa las credenciales y lo envía.
function submitLogin(email: string, password: string) {
  render(
    <SessionProvider>
      <LoginForm />
    </SessionProvider>,
  );
  fireEvent.change(screen.getByLabelText(/correo electrónico/i), {
    target: { value: email },
  });
  fireEvent.change(screen.getByLabelText(/contraseña/i), {
    target: { value: password },
  });
  fireEvent.click(screen.getByRole("button", { name: /iniciar sesión/i }));
}

describe("001/US2 - Inicio de sesión", () => {
  // Cada escenario parte de una pestaña sin sesión y sin navegaciones previas.
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Credenciales válidas: se envían al endpoint de inicio de sesión, el token
  // queda guardado en sessionStorage y la persona llega al área protegida.
  it("Caso normal — inicio de sesión con credenciales válidas", async () => {
    const fetchMock = stubFetch(200, {
      token: issuedToken,
      expiresAt: "2026-10-10T20:00:00Z",
    });

    submitLogin("ana@example.com", validPassword);

    await waitFor(() => expect(navigations()).toContain("/profile"));
    const request = await sentRequest(fetchMock);
    expect(request.url.endsWith("/api/v1/auth/login")).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.body).toEqual({
      email: "ana@example.com",
      password: validPassword,
    });
    expect(storedValues()).toContain(issuedToken);
  });

  // El correo con espacios extremos y mayúsculas no se rechaza en el
  // formulario: llega al backend como el mismo correo y el inicio es exitoso.
  it("Caso alternativo — correo con espacios y mayúsculas", async () => {
    const fetchMock = stubFetch(200, {
      token: issuedToken,
      expiresAt: "2026-10-10T20:00:00Z",
    });

    submitLogin("  ANA@Example.com  ", validPassword);

    await waitFor(() => expect(navigations()).toContain("/profile"));
    const request = await sentRequest(fetchMock);
    expect(request.body.email.trim().toLowerCase()).toBe("ana@example.com");
    expect(storedValues()).toContain(issuedToken);
    expect(screen.queryByText(genericError)).toBeNull();
  });

  // Ante el 401 por correo inexistente se muestra el mensaje genérico, no se
  // guarda ninguna sesión y la persona permanece en el formulario.
  it("Caso de error — correo inexistente con cualquier contraseña", async () => {
    stubFetch(401, { error: genericError });

    submitLogin("nadie@example.com", "Cualquiera123");

    expect(await screen.findByText(genericError)).toBeDefined();
    expect(storedValues()).toHaveLength(0);
    expect(navigations()).toHaveLength(0);
  });

  // Ante el 401 por contraseña incorrecta se muestra exactamente el mismo
  // mensaje que con el correo inexistente, sin revelar que la cuenta existe.
  it("Caso de error — contraseña incorrecta con el mismo mensaje del correo inexistente", async () => {
    stubFetch(401, { error: genericError });

    submitLogin("ana@example.com", "Incorrecta2026");

    expect(await screen.findByText(genericError)).toBeDefined();
    expect(storedValues()).toHaveLength(0);
    expect(navigations()).toHaveLength(0);
  });
});

// Aviso explícito que muestra la pantalla de inicio de sesión cuando llega
// desde una acción rechazada por sesión inválida (FR-016).
const sessionNotice =
  "Tu sesión no es válida o venció. Volvé a iniciar sesión para continuar.";

// Monta la pantalla de inicio de sesión dentro del proveedor de sesión, con la
// cadena de consulta indicada en la URL.
function renderLoginAt(search: string) {
  currentSearch.value = search;
  render(
    <SessionProvider>
      <LoginForm />
    </SessionProvider>,
  );
}

describe("001/US3 - Protección de las funciones del sistema", () => {
  // Cada escenario parte de una pestaña sin sesión y con la URL sin consulta.
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
    currentSearch.value = "";
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    currentSearch.value = "";
  });

  // La guardia llevó a /login?motivo=sesion porque la API rechazó la sesión:
  // la pantalla informa de forma explícita que hay que volver a iniciar sesión.
  it("Caso límite — la sesión vence entre una acción y la siguiente: el inicio de sesión muestra el aviso", () => {
    renderLoginAt("motivo=sesion");

    expect(screen.getByText(sessionNotice)).toBeDefined();
  });

  // El visitante que entra a /login sin motivo usa el inicio de sesión con
  // normalidad, sin un aviso de sesión vencida que no le corresponde.
  it("Caso alternativo — visitante sin sesión accede al inicio de sesión: no muestra el aviso", () => {
    renderLoginAt("");

    expect(screen.queryByText(sessionNotice)).toBeNull();
    expect(
      screen.getByRole("button", { name: /iniciar sesión/i }),
    ).toBeDefined();
  });
});
