import { afterEach, describe, expect, it, vi } from "vitest";
import {
  defaultProjectId,
  installDashboardBackend,
} from "@/features/dashboard/components/dashboard-test-backend";

// Pruebas de caracterización del backend de prueba del dashboard (T048, per T040): fetch puede
// recibir un string, un URL o un Request, y el mock debe registrar la ruta en los tres casos.

// Verifica que el backend de prueba registre la ruta segÃºn el tipo de entrada de fetch.
describe("dashboard - backend de prueba (T048)", () => {
  // Restablece el fetch global sustituido.
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("registra la ruta y responde cuando fetch recibe un URL", async () => {
    const requests = installDashboardBackend();
    const url = new URL(`http://localhost/api/v1/projects/${defaultProjectId}`);

    const response = await fetch(url);

    expect(response.status).toBe(200);
    expect(requests).toEqual([
      { method: "GET", path: `/api/v1/projects/${defaultProjectId}` },
    ]);
  });

  it("registra la ruta y responde cuando fetch recibe un Request", async () => {
    const requests = installDashboardBackend();
    const request = new Request(
      `http://localhost/api/v1/projects/${defaultProjectId}/metrics`,
    );

    const response = await fetch(request);

    expect(response.status).toBe(200);
    expect(requests).toEqual([
      { method: "GET", path: `/api/v1/projects/${defaultProjectId}/metrics` },
    ]);
  });
});
