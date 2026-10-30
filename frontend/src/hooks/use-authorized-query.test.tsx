import { cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiResult } from "@/lib/api-client";
import { SessionProvider } from "@/providers/session-provider";
import { writeToken } from "@/lib/session-storage";
import { useAuthorizedQuery } from "./use-authorized-query";

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador montado.
const router = vi.hoisted(() => ({
  push: vi.fn<(href: string) => void>(),
  replace: vi.fn<(href: string) => void>(),
  refresh: vi.fn<() => void>(),
  back: vi.fn<() => void>(),
  forward: vi.fn<() => void>(),
  prefetch: vi.fn<(href: string) => void>(),
}));

// Hace que useRouter devuelva el enrutador sustituido.
vi.mock("next/navigation", () => ({
  useRouter: () => router,
}));

// Token de sesiÃ³n vigente para las pruebas.
const issuedToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";

// Resultado de error de la API con el código HTTP y el mensaje indicados.
function failure(status: number, message: string): ApiResult<string> {
  return { ok: false, error: { status, message, fields: [] } };
}

// Pantalla de prueba: vuelca el estado que devuelve el hook como JSON, de modo que la prueba
// observe el contrato completo del estado sin depender de los tipos todavía no ampliados.
function QueryProbe({ load }: { load: () => Promise<ApiResult<string>> }) {
  const state = useAuthorizedQuery(load);
  return <output data-testid="state">{JSON.stringify(state)}</output>;
}

// Monta la sonda con sesión vigente y devuelve el estado final ya resuelto.
async function settledState(result: ApiResult<string>): Promise<unknown> {
  writeToken(issuedToken);
  const load = () => Promise.resolve(result);
  const view = render(
    <SessionProvider>
      <QueryProbe load={load} />
    </SessionProvider>,
  );
  await waitFor(() => {
    expect(view.getByTestId("state").textContent).not.toContain("loading");
  });
  return JSON.parse(view.getByTestId("state").textContent ?? "null");
}

// Verifica el estado que devuelve el hook ante errores de la API y sesiÃ³n vencida.
describe("Usar el dashboard cuando falta información o falla una sección", () => {
  // Cada prueba arranca sin sesiÃ³n y con los espÃ­as limpios.
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  // Desmonta la sonda renderizada tras cada prueba.
  afterEach(() => {
    cleanup();
  });

  // Escenario: una sección falla con un error del servidor (research.md sección 4).
  it("el estado de error trae el código HTTP 500 y el mensaje", async () => {
    const state = await settledState(failure(500, "Error interno."));

    expect(state).toEqual({
      status: "error",
      httpStatus: 500,
      message: "Error interno.",
    });
  });

  // Escenario: la red está caída y el cliente HTTP informa status 0.
  it("el estado de error trae el código HTTP 0 con la red caída", async () => {
    const state = await settledState(failure(0, "Sin conexión."));

    expect(state).toEqual({
      status: "error",
      httpStatus: 0,
      message: "Sin conexión.",
    });
  });

  // Escenario: con datos, el estado cargado no cambia respecto de hoy.
  it("con datos el estado sigue siendo loaded con la data", async () => {
    const state = await settledState({ ok: true, data: "hola" });

    expect(state).toEqual({ status: "loaded", data: "hola" });
  });
});
