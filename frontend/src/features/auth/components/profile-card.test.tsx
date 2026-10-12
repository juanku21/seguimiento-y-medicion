import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import { ProfileCard } from "./profile-card";

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador
// montado, y así la tarjeta puede usarlo sin fallar si lo necesita.
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

// Token de la sesión vigente y respuestas del contrato GET /api/v1/users/me
// (contracts/openapi.yaml, UserProfile y respuesta SesionInvalida).
const issuedToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const invalidSessionMessage = "Se requiere iniciar sesión.";
const profilePath = "/api/v1/users/me";
const ownProfile = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
  email: "ana@example.com",
};

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

// Extrae URL, método y cabecera Authorization de la petición que envió la
// tarjeta, tanto si se llamó a fetch con una URL y opciones como con un Request.
function sentRequest(fetchMock: ReturnType<typeof stubFetch>) {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const [input, init] = fetchMock.mock.calls[0];
  if (input instanceof Request) {
    return {
      url: input.url,
      method: input.method,
      authorization: input.headers.get("Authorization"),
    };
  }
  return {
    url: input instanceof URL ? input.href : input,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

// Devuelve los valores guardados en sessionStorage, sin depender del nombre
// de la clave que elija la implementación (research.md, sección 9).
function storedValues(): (string | null)[] {
  return Array.from({ length: sessionStorage.length }, (_, i) =>
    sessionStorage.getItem(sessionStorage.key(i) ?? ""),
  );
}

// Monta la tarjeta de perfil dentro del proveedor de sesión, como en el
// layout raíz.
function renderProfileCard() {
  return render(
    <SessionProvider>
      <ProfileCard />
    </SessionProvider>,
  );
}

describe("001/US4 - Consulta del propio perfil", () => {
  // Cada escenario parte de una pestaña sin sesión y sin dobles previos.
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Con sesión vigente, la tarjeta pide el perfil propio con el token de la
  // pestaña, sin ningún identificador de cuenta (FR-022, FR-023), y muestra el
  // nombre completo y el correo del titular.
  it("Caso normal — usuario con sesión vigente consulta su perfil", async () => {
    writeToken(issuedToken);
    const fetchMock = stubFetch(200, ownProfile);

    renderProfileCard();

    expect(await screen.findByText(ownProfile.fullName)).toBeDefined();
    expect(screen.getByText(ownProfile.email)).toBeDefined();
    const request = sentRequest(fetchMock);
    expect(request.url.endsWith(profilePath)).toBe(true);
    expect(request.method).toBe("GET");
    expect(request.authorization).toBe(`Bearer ${issuedToken}`);
  });

  // El correo se registró como "  Ana@Example.COM  ": la tarjeta muestra el
  // valor normalizado que devuelve la API, el mismo con el que se inicia
  // sesión, sin alterarlo ni mostrar la forma original.
  it("Caso alternativo — correo registrado con espacios y mayúsculas se ve normalizado", async () => {
    writeToken(issuedToken);
    stubFetch(200, ownProfile);

    const view = renderProfileCard();

    expect(await screen.findByText("ana@example.com")).toBeDefined();
    expect(view.container.textContent).not.toContain("Ana@Example.COM");
  });

  // Aunque la respuesta trajera campos de más, la tarjeta solo expone el
  // nombre y el correo: ni la contraseña ni ninguna representación derivada
  // de ella llegan a la interfaz (FR-011, SC-005).
  it("Caso límite — la respuesta inspeccionada no muestra la contraseña ni nada derivado", async () => {
    const leakedPassword = "Proyecto2026";
    const leakedHash =
      "$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW";
    writeToken(issuedToken);
    stubFetch(200, {
      ...ownProfile,
      password: leakedPassword,
      passwordHash: leakedHash,
    });

    const view = renderProfileCard();

    expect(await screen.findByText(ownProfile.fullName)).toBeDefined();
    const shown = view.container.textContent ?? "";
    expect(shown).not.toContain(leakedPassword);
    expect(shown).not.toContain(leakedHash);
    expect(shown).not.toMatch(/password|hash/i);
  });

  // La API rechaza la sesión con 401: la tarjeta no muestra datos de ninguna
  // cuenta y el cliente HTTP descarta el token de la pestaña (SC-003, FR-016).
  it("Caso de error — visitante sin sesión válida consulta el perfil", async () => {
    writeToken(issuedToken);
    const fetchMock = stubFetch(401, { error: invalidSessionMessage });

    const view = renderProfileCard();

    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    await waitFor(() => expect(storedValues()).not.toContain(issuedToken));
    const shown = view.container.textContent ?? "";
    expect(shown).not.toContain(ownProfile.fullName);
    expect(shown).not.toContain(ownProfile.email);
  });
});
