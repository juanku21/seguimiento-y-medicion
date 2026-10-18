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
  within,
} from "@testing-library/react";
import { MemberList } from "@/features/projects/components/member-list";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";
import type { ProjectMemberView } from "@/types/project";

// Pruebas de la lista de integrantes (T046). Atraviesan el componente, el formulario de alta que
// incluye y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el backend. Qué
// campos llegan (el correo solo para el propietario) y las reglas de negocio las resuelve el
// backend (T043 y T044): acá se verifica qué muestra la vista y qué acciones ofrece.
//
// Contrato que fijan estas pruebas para la lista:
// - Recibe projectId, los integrantes tal como los devuelve la API y viewerIsOwner.
// - Los integrantes se muestran en una lista con nombre accesible "Integrantes", un elemento por
//   persona con su nombre completo, la marca "Propietario" solo en el propietario, la fecha de
//   incorporación (AAAA-MM-DD o DD/MM/AAAA) y el correo solo cuando consulta el propietario.
// - Solo si viewerIsOwner es true se ofrecen las acciones de administración: el formulario de alta
//   (campo "Correo electrónico" y botón "Agregar integrante") y un botón "Quitar" en cada
//   integrante que no es el propietario.
// - Tras un alta o una baja exitosa la lista se actualiza sin repetir a nadie; ante un error se
//   muestra el mensaje del backend con role="alert" y la lista no cambia.

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const membersPath = `/api/v1/projects/${projectId}/members`;

// Integrantes de "Portal de Clientes" tal como los ve la propietaria, Ana.
const ana: ProjectMemberView = {
  userId: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
  email: "ana@example.com",
  isOwner: true,
  joinedAt: "2026-10-01T12:00:00Z",
};
const bruno: ProjectMemberView = {
  userId: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
  fullName: "Bruno Gómez",
  email: "bruno@example.com",
  isOwner: false,
  joinedAt: "2026-10-03T12:00:00Z",
};
const carla: ProjectMemberView = {
  userId: "5a9e3c71-4f28-4b06-8d1e-6c2b7f0a9e13",
  fullName: "Carla Ruiz",
  email: "carla@example.com",
  isOwner: false,
  joinedAt: "2026-10-05T12:00:00Z",
};

// Bruno tal como vuelve en el 201 de una nueva alta, con una fecha de incorporación nueva.
const brunoReAdded: ProjectMemberView = {
  ...bruno,
  joinedAt: "2026-10-08T12:00:00Z",
};

// Quita el correo, como hace el backend cuando quien consulta no es el propietario (FR-030).
function withoutEmail(member: ProjectMemberView): ProjectMemberView {
  const { email, ...rest } = member;
  void email;
  return rest;
}

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ProjectMemberView | ErrorResponse) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la URL, el método y la cabecera Authorization de la llamada número index a fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const authorization = new Headers(init?.headers).get("Authorization");
  return { url, method: init?.method, authorization };
}

// Indica si el texto muestra la fecha de un instante ISO como AAAA-MM-DD o como DD/MM/AAAA.
function showsDate(text: string, isoDateTime: string): boolean {
  const isoDate = isoDateTime.slice(0, 10);
  const [year, month, day] = isoDate.split("-");
  return text.includes(isoDate) || text.includes(`${day}/${month}/${year}`);
}

// Devuelve los elementos de la lista de integrantes, esperando a que se muestre.
async function memberItems(): Promise<HTMLElement[]> {
  const list = await screen.findByRole("list", { name: /integrantes/i });
  return within(list).getAllByRole("listitem");
}

// Devuelve el elemento de la lista que corresponde a la persona indicada.
async function memberRow(fullName: string): Promise<HTMLElement> {
  const row = (await memberItems()).find((item) =>
    (item.textContent ?? "").includes(fullName),
  );
  if (row === undefined) {
    throw new Error(`No se muestra a ${fullName} en la lista de integrantes`);
  }
  return row;
}

// Cuenta cuántas veces aparece la persona indicada en la lista de integrantes.
async function occurrencesOf(fullName: string): Promise<number> {
  return (await memberItems()).filter((item) =>
    (item.textContent ?? "").includes(fullName),
  ).length;
}

// Agrega un integrante con el formulario de alta que ofrece la lista.
function addByEmail(email: string) {
  fireEvent.change(screen.getByLabelText(/correo electrónico/i), {
    target: { value: email },
  });
  fireEvent.click(screen.getByRole("button", { name: /agregar integrante/i }));
}

// Pulsa la acción de quitar de la persona indicada.
async function removeMember(fullName: string) {
  const row = await memberRow(fullName);
  fireEvent.click(within(row).getByRole("button", { name: /quitar/i }));
}

describe("002/US3 - Gestionar los integrantes del proyecto", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que acepta las bajas (204) y
  // las altas (201 con Bruno).
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(async (_input, init) =>
      init?.method === "DELETE"
        ? new Response(null, { status: 204 })
        : jsonResponse(201, bruno),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: el propietario agrega a Bruno con el formulario de la lista y Bruno pasa a
  // mostrarse como integrante.
  it("Alta exitosa", async () => {
    render(
      <MemberList projectId={projectId} members={[ana]} viewerIsOwner={true} />,
    );
    addByEmail("bruno@example.com");

    await waitFor(async () =>
      expect(await occurrencesOf("Bruno Gómez")).toBe(1),
    );
    expect(await memberItems()).toHaveLength(2);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(membersPath)).toBe(true);
    expect(request.method).toBe("POST");
  });

  // Caso alternativo: un integrante quitado y vuelto a agregar recupera su lugar y aparece una sola
  // vez en la lista (FR-029).
  it("Nueva alta tras una baja", async () => {
    fetchMock.mockImplementation(async (_input, init) =>
      init?.method === "DELETE"
        ? new Response(null, { status: 204 })
        : jsonResponse(201, brunoReAdded),
    );
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno]}
        viewerIsOwner={true}
      />,
    );

    await removeMember("Bruno Gómez");
    await waitFor(async () =>
      expect(await occurrencesOf("Bruno Gómez")).toBe(0),
    );

    addByEmail("bruno@example.com");
    await waitFor(async () =>
      expect(await occurrencesOf("Bruno Gómez")).toBe(1),
    );
    expect(await memberItems()).toHaveLength(2);
    const row = await memberRow("Bruno Gómez");
    expect(showsDate(row.textContent ?? "", brunoReAdded.joinedAt)).toBe(true);
  });

  // Caso alternativo: el propietario ve de cada persona su nombre, su correo, la marca de
  // propietario y la fecha de incorporación; un integrante que no es el propietario ve lo mismo
  // salvo los correos, y no ve ninguna acción de administración (FR-030).
  it("Visibilidad del correo", async () => {
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno, carla]}
        viewerIsOwner={true}
      />,
    );
    for (const member of [ana, bruno, carla]) {
      const text = (await memberRow(member.fullName)).textContent ?? "";
      expect(text, member.fullName).toContain(member.email);
      expect(showsDate(text, member.joinedAt), member.fullName).toBe(true);
    }
    expect((await memberRow("Ana Pérez")).textContent).toMatch(/propietario/i);
    expect((await memberRow("Bruno Gómez")).textContent).not.toMatch(
      /propietario/i,
    );
    cleanup();

    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno, carla].map(withoutEmail)}
        viewerIsOwner={false}
      />,
    );
    const items = await memberItems();
    expect(items).toHaveLength(3);
    for (const member of [ana, bruno, carla]) {
      const text = (await memberRow(member.fullName)).textContent ?? "";
      expect(showsDate(text, member.joinedAt), member.fullName).toBe(true);
    }
    expect((await memberRow("Ana Pérez")).textContent).toMatch(/propietario/i);
    expect(document.body.textContent).not.toContain("@");
    expect(screen.queryByRole("button", { name: /quitar/i })).toBeNull();
    expect(screen.queryByLabelText(/correo electrónico/i)).toBeNull();
    expect(
      screen.queryByRole("button", { name: /agregar integrante/i }),
    ).toBeNull();
  });

  // Caso límite: con el propietario como único integrante, la lista no está vacía y lo muestra
  // marcado como propietario.
  it("Solo el propietario como integrante", async () => {
    render(
      <MemberList projectId={projectId} members={[ana]} viewerIsOwner={true} />,
    );

    const items = await memberItems();
    expect(items).toHaveLength(1);
    expect(items[0].textContent).toContain("Ana Pérez");
    expect(items[0].textContent).toMatch(/propietario/i);
  });

  // Caso límite: la baja envía DELETE con la sesión de la pestaña y solo quita a la persona de la
  // lista; los demás integrantes siguen. Que sus registros se conserven lo verifica el backend
  // (TestUS3_BajaConservaElHistorial).
  it("Baja que conserva el historial", async () => {
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno, carla]}
        viewerIsOwner={true}
      />,
    );

    await removeMember("Bruno Gómez");
    await waitFor(async () =>
      expect(await occurrencesOf("Bruno Gómez")).toBe(0),
    );
    expect(await occurrencesOf("Ana Pérez")).toBe(1);
    expect(await occurrencesOf("Carla Ruiz")).toBe(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith(`${membersPath}/${bruno.userId}`)).toBe(true);
    expect(request.method).toBe("DELETE");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  });

  // Caso de error: el alta de alguien que ya es integrante se rechaza con el mensaje del backend y
  // la lista no cambia (FR-022).
  it("Ya es integrante", async () => {
    const message = "La persona ya forma parte del proyecto.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: message }),
    );
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno]}
        viewerIsOwner={true}
      />,
    );
    addByEmail("bruno@example.com");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(await memberItems()).toHaveLength(2);
    expect(await occurrencesOf("Bruno Gómez")).toBe(1);
  });

  // Caso de error: un integrante que no es el propietario ve la lista pero no tiene forma de
  // agregar ni de quitar a nadie, y no ve los correos (FR-030, FR-047).
  it("No es propietario", async () => {
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno, carla].map(withoutEmail)}
        viewerIsOwner={false}
      />,
    );

    expect(await memberItems()).toHaveLength(3);
    expect(screen.queryByRole("button", { name: /quitar/i })).toBeNull();
    expect(screen.queryByLabelText(/correo electrónico/i)).toBeNull();
    expect(
      screen.queryByRole("button", { name: /agregar integrante/i }),
    ).toBeNull();
    expect(document.body.textContent).not.toContain("@");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso de error: la lista no ofrece al propietario la acción de quitarse a sí mismo, mientras que
  // sí la ofrece en los demás integrantes (FR-025, RN-12).
  it("Propietario no puede quitarse", async () => {
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno]}
        viewerIsOwner={true}
      />,
    );

    const brunoRow = await memberRow("Bruno Gómez");
    expect(
      within(brunoRow).getByRole("button", { name: /quitar/i }),
    ).toBeTruthy();
    const anaRow = await memberRow("Ana Pérez");
    expect(
      within(anaRow).queryByRole("button", { name: /quitar/i }),
    ).toBeNull();
  });

  // Caso de error: sobre un proyecto Finalizado la baja se rechaza con el mensaje del backend y la
  // persona sigue en la lista (FR-018, RN-11).
  it("Proyecto finalizado", async () => {
    const message = "El proyecto está finalizado y es de solo lectura.";
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: message }),
    );
    render(
      <MemberList
        projectId={projectId}
        members={[ana, bruno]}
        viewerIsOwner={true}
      />,
    );

    await removeMember("Bruno Gómez");
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(await occurrencesOf("Bruno Gómez")).toBe(1);
    expect(await memberItems()).toHaveLength(2);
  });
});
