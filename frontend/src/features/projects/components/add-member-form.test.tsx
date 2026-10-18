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
  waitFor,
} from "@testing-library/react";
import { AddMemberForm } from "@/features/projects/components/add-member-form";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";
import type { ProjectMemberView } from "@/types/project";

// Pruebas del formulario de alta de integrantes (T046). Atraviesan el componente y el cliente HTTP
// real; solo se sustituye fetch, que es la frontera con el backend. La normalización del correo y
// las reglas de negocio las resuelve el backend (T043 y T044): acá se verifica qué envía el
// formulario y cómo muestra cada respuesta.
//
// Contrato que fijan estas pruebas para el formulario:
// - Recibe projectId y onAdded; el campo se encuentra por la etiqueta "Correo electrónico" y el
//   envío por el botón "Agregar integrante".
// - Envía POST /api/v1/projects/{projectId}/members con la sesión de la pestaña y el cuerpo
//   { email }.
// - Ante un 201 llama a onAdded con el integrante recibido; ante un error muestra el mensaje del
//   backend con role="alert" y no llama a onAdded.

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const membersPath = `/api/v1/projects/${projectId}/members`;

const addedBruno: ProjectMemberView = {
  userId: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
  fullName: "Bruno Gómez",
  email: "bruno@example.com",
  isOwner: false,
  joinedAt: "2026-10-03T09:15:00Z",
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ProjectMemberView | ErrorResponse) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la URL, el método, la cabecera Authorization y el cuerpo JSON de la única llamada a fetch.
function sentRequest(fetchMock: FetchMock) {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const [input, init] = fetchMock.mock.calls[0];
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const authorization = new Headers(init?.headers).get("Authorization");
  const body = JSON.parse(String(init?.body)) as { email: string };
  return { url, method: init?.method, authorization, body };
}

// Escribe el correo indicado en el formulario y lo envía.
function submitEmail(email: string) {
  fireEvent.change(screen.getByLabelText(/correo electrónico/i), {
    target: { value: email },
  });
  fireEvent.click(screen.getByRole("button", { name: /agregar integrante/i }));
}

describe("002/US3 - Gestionar los integrantes del proyecto", () => {
  let fetchMock: FetchMock;
  let onAdded: Mock<(member: ProjectMemberView) => void>;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que responde 201 con Bruno.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(async () => jsonResponse(201, addedBruno));
    vi.stubGlobal("fetch", fetchMock);
    onAdded = vi.fn<(member: ProjectMemberView) => void>();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: el formulario envía el alta con la sesión de la pestaña y entrega el integrante
  // recibido, sin mostrar ningún error.
  it("Alta exitosa", async () => {
    render(<AddMemberForm projectId={projectId} onAdded={onAdded} />);
    submitEmail("bruno@example.com");

    await waitFor(() => expect(onAdded).toHaveBeenCalledTimes(1));
    expect(onAdded).toHaveBeenCalledWith(addedBruno);
    const request = sentRequest(fetchMock);
    expect(request.url.endsWith(membersPath)).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toEqual({ email: "bruno@example.com" });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: un correo con espacios extremos y mayúsculas no se rechaza en el formulario;
  // se envía y, normalizado, corresponde a la misma cuenta (FR-020, RN-05).
  it("Correo con espacios y mayúsculas", async () => {
    render(<AddMemberForm projectId={projectId} onAdded={onAdded} />);
    submitEmail("  Bruno@Example.COM  ");

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(addedBruno));
    const { body } = sentRequest(fetchMock);
    expect(body.email.trim().toLowerCase()).toBe("bruno@example.com");
  });

  // Caso de error: el correo no corresponde a ninguna cuenta (422) y el formulario lo informa sin
  // agregar a nadie (FR-021).
  it("Correo sin cuenta", async () => {
    const message =
      "No existe un usuario registrado con ese correo electrónico.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(422, { error: message }),
    );
    render(<AddMemberForm projectId={projectId} onAdded={onAdded} />);
    submitEmail("nadie@example.com");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(sentRequest(fetchMock).body).toEqual({ email: "nadie@example.com" });
    expect(onAdded).not.toHaveBeenCalled();
  });

  // Caso de error: la persona ya es integrante (409) y el formulario lo informa sin agregarla de
  // nuevo (FR-022).
  it("Ya es integrante", async () => {
    const message = "La persona ya forma parte del proyecto.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: message }),
    );
    render(<AddMemberForm projectId={projectId} onAdded={onAdded} />);
    submitEmail("bruno@example.com");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(onAdded).not.toHaveBeenCalled();
  });

  // Caso de error: quien pide no es el propietario (403) y el alta se rechaza por falta de
  // permiso, sin ningún efecto (FR-047, SC-006).
  it("No es propietario", async () => {
    const message =
      "Solo el propietario del proyecto puede realizar esta acción.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(403, { error: message }),
    );
    render(<AddMemberForm projectId={projectId} onAdded={onAdded} />);
    submitEmail("bruno@example.com");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(onAdded).not.toHaveBeenCalled();
  });

  // Caso de error: el proyecto está Finalizado (409) y el alta se rechaza por ser de solo lectura
  // (FR-018, RN-11).
  it("Proyecto finalizado", async () => {
    const message = "El proyecto está finalizado y es de solo lectura.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: message }),
    );
    render(<AddMemberForm projectId={projectId} onAdded={onAdded} />);
    submitEmail("bruno@example.com");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(onAdded).not.toHaveBeenCalled();
  });
});
