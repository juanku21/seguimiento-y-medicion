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
import { RegisterForm } from "@/features/auth/components/register-form";
import type {
  ErrorResponse,
  RegisterRequest,
  UserProfile,
  ValidationErrorResponse,
} from "@/types/auth";

// Pruebas del formulario de registro (T028). Atraviesan el componente, la validación del
// formulario y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el backend.
//
// Contrato de accesibilidad que fijan estas pruebas para el formulario:
// - Cada campo se encuentra por su etiqueta: "Nombre completo", "Correo electrónico", "Contraseña".
// - La indicación del límite de la contraseña y los errores de cada campo se asocian al campo con
//   aria-describedby; un campo con error lleva aria-invalid="true".
// - Los errores generales (por ejemplo, correo no disponible) se muestran con role="alert".

type FetchMock = Mock<typeof fetch>;

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body: UserProfile | ErrorResponse | ValidationErrorResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Devuelve el texto de todos los elementos que describen a un campo vía aria-describedby.
function describedText(element: HTMLElement): string {
  const ids = (element.getAttribute("aria-describedby") ?? "")
    .split(/\s+/)
    .filter(Boolean);
  return ids
    .map((id) => document.getElementById(id)?.textContent ?? "")
    .join(" ");
}

// Localiza los campos y el botón de envío del formulario.
function getFields() {
  return {
    fullName: screen.getByLabelText(/nombre completo/i),
    email: screen.getByLabelText(/correo electrónico/i),
    password: screen.getByLabelText(/contraseña/i),
    submit: screen.getByRole("button", { name: /crear cuenta|registr/i }),
  };
}

// Completa el formulario con los valores dados y lo envía.
function fillAndSubmit(values: RegisterRequest) {
  const fields = getFields();
  fireEvent.change(fields.fullName, { target: { value: values.fullName } });
  fireEvent.change(fields.email, { target: { value: values.email } });
  fireEvent.change(fields.password, { target: { value: values.password } });
  fireEvent.click(fields.submit);
  return fields;
}

// Extrae la URL, el método y el cuerpo JSON de la llamada número index a fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const body = JSON.parse(String(init?.body)) as RegisterRequest;
  return { url, method: init?.method, body };
}

const validInput: RegisterRequest = {
  fullName: "Ana Pérez",
  email: "ana@example.com",
  password: "Proyecto2026",
};

const createdProfile: UserProfile = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
  email: "ana@example.com",
};

describe("001/US1 - Registro de cuenta", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con un fetch que responde 201 y con el almacenamiento vacío.
  beforeEach(() => {
    fetchMock = vi.fn<typeof fetch>(async () =>
      jsonResponse(201, createdProfile),
    );
    vi.stubGlobal("fetch", fetchMock);
    sessionStorage.clear();
    localStorage.clear();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: el formulario envía POST /api/v1/auth/register y confirma el alta.
  it("Alta exitosa", async () => {
    render(<RegisterForm />);
    fillAndSubmit(validInput);

    expect(await screen.findByText(/cuenta creada/i)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.url.endsWith("/api/v1/auth/register")).toBe(true);
    expect(request.method).toBe("POST");
    expect(request.body).toEqual(validInput);
  });

  // Caso alternativo: el correo se envía recortado y en minúsculas.
  it("Correo normalizado", async () => {
    render(<RegisterForm />);
    fillAndSubmit({ ...validInput, email: "  Ana@Example.COM  " });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(requestAt(fetchMock, 0).body.email).toBe("ana@example.com");
  });

  // RC-02: el límite de 8 a 72 caracteres se comunica junto al campo desde el inicio, sin haber
  // enviado nada y sin que el campo esté marcado como inválido.
  it("muestra el límite de 8 a 72 caracteres antes de enviar, no al fallar", () => {
    render(<RegisterForm />);
    const { password } = getFields();

    const hint = describedText(password);
    expect(hint).toMatch(/8/);
    expect(hint).toMatch(/72/);
    expect(hint).toMatch(/caracteres/i);
    expect(password.getAttribute("aria-invalid")).not.toBe("true");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso límite: 8 y 72 caracteres se envían; un emoji cuenta como un solo carácter, así que
  // 6 emojis más una letra y un número (8 caracteres) también se envían.
  it("Límites de contraseña", async () => {
    const passwords = ["abcdefg1", "a".repeat(71) + "1", "😀".repeat(6) + "a1"];
    for (const [index, password] of passwords.entries()) {
      render(<RegisterForm />);
      fillAndSubmit({ ...validInput, password });
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(index + 1));
      expect(requestAt(fetchMock, index).body.password).toBe(password);
      cleanup();
    }
  });

  // Un emoji cuenta como un solo carácter: 5 emojis más una letra y un número son 7 caracteres
  // (aunque ocupen 12 unidades UTF-16) y se rechazan antes de enviar.
  it("Límites de contraseña: un emoji cuenta como un solo carácter", async () => {
    render(<RegisterForm />);
    const { password } = fillAndSubmit({
      ...validInput,
      password: "😀".repeat(5) + "a1",
    });

    await waitFor(() =>
      expect(password.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(password)).toMatch(/72/);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Una contraseña de 41 caracteres (40 letras acentuadas y un número) ocupa 81 bytes en UTF-8:
  // se rechaza antes de enviar con el mensaje del rango permitido.
  it("rechaza antes de enviar una contraseña de 72 caracteres o menos que supera los 72 bytes en UTF-8", async () => {
    render(<RegisterForm />);
    const { password } = fillAndSubmit({
      ...validInput,
      password: "á".repeat(40) + "1",
    });

    await waitFor(() =>
      expect(password.getAttribute("aria-invalid")).toBe("true"),
    );
    const description = describedText(password);
    expect(description).toMatch(/8/);
    expect(description).toMatch(/72/);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso límite: nombres de 2 y de 100 caracteres, contados tras recortar espacios, se envían.
  it("Límites de nombre", async () => {
    const names = ["  Al  ", "a".repeat(100)];
    for (const [index, fullName] of names.entries()) {
      render(<RegisterForm />);
      fillAndSubmit({ ...validInput, fullName });
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(index + 1));
      cleanup();
    }
  });

  // Concurrencia: un doble clic mientras el primer envío está en curso no genera un segundo envío.
  it("Registro concurrente", async () => {
    let resolveRequest: (response: Response) => void = () => {};
    fetchMock.mockImplementation(
      () => new Promise<Response>((resolve) => (resolveRequest = resolve)),
    );
    render(<RegisterForm />);
    const { submit } = fillAndSubmit(validInput);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));

    fireEvent.click(submit);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    resolveRequest(jsonResponse(201, createdProfile));
    expect(await screen.findByText(/cuenta creada/i)).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  // Caso alternativo: el registro no deja sesión guardada y ofrece ir a iniciar sesión.
  it("Registro no emite sesión", async () => {
    render(<RegisterForm />);
    fillAndSubmit(validInput);

    await screen.findByText(/cuenta creada/i);
    expect(sessionStorage.length).toBe(0);
    expect(localStorage.length).toBe(0);
    const loginLink = screen.getByRole("link", { name: /iniciar sesión/i });
    expect(loginLink.getAttribute("href")).toBe("/login");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  // Caso de error: el backend responde 409 y el formulario muestra que el correo no está
  // disponible, sin confirmar el alta.
  it("Correo duplicado", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(409, { error: "El correo electrónico no está disponible." }),
    );
    render(<RegisterForm />);
    fillAndSubmit({ ...validInput, email: "ANA@example.com" });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(
      "El correo electrónico no está disponible.",
    );
    expect(screen.queryByText(/cuenta creada/i)).toBeNull();
  });

  // Caso de error: con nombre y correo ausentes no se envía nada y cada campo indica qué corregir.
  it("Entrada inválida: nombre y correo ausentes", async () => {
    render(<RegisterForm />);
    const { fullName, email } = fillAndSubmit({
      ...validInput,
      fullName: "   ",
      email: "",
    });

    await waitFor(() =>
      expect(fullName.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fullName)).toMatch(/nombre/i);
    expect(email.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(email)).toMatch(/correo/i);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso de error: un correo sin formato válido no se envía.
  it("Entrada inválida: correo con formato inválido", async () => {
    render(<RegisterForm />);
    const { email } = fillAndSubmit({
      ...validInput,
      email: "ana.example.com",
    });

    await waitFor(() =>
      expect(email.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(email)).toMatch(/correo/i);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso de error: contraseñas de 7 y de 73 caracteres se rechazan antes de enviar.
  it("Entrada inválida: contraseña de 7 o de 73 caracteres", async () => {
    for (const value of ["abcdef1", "a".repeat(72) + "1"]) {
      render(<RegisterForm />);
      const { password } = fillAndSubmit({ ...validInput, password: value });
      await waitFor(() =>
        expect(password.getAttribute("aria-invalid")).toBe("true"),
      );
      expect(describedText(password)).toMatch(/72/);
      cleanup();
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso de error: contraseñas sin letra o sin número indican la composición requerida.
  it("Entrada inválida: contraseña sin letra o sin número", async () => {
    for (const value of ["12345678", "abcdefgh"]) {
      render(<RegisterForm />);
      const { password } = fillAndSubmit({ ...validInput, password: value });
      await waitFor(() =>
        expect(password.getAttribute("aria-invalid")).toBe("true"),
      );
      const description = describedText(password);
      expect(description).toMatch(/letra/i);
      expect(description).toMatch(/número/i);
      cleanup();
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // Caso de error: si el backend rechaza un campo con 400, el mensaje aparece junto a ese campo.
  it("Entrada inválida: muestra el detalle por campo que devuelve el backend", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(400, {
        error: "Hay campos inválidos en la solicitud.",
        fields: [
          {
            field: "email",
            message: "El formato del correo electrónico no es válido.",
          },
        ],
      }),
    );
    render(<RegisterForm />);
    const { email } = fillAndSubmit(validInput);

    await waitFor(() =>
      expect(email.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(email)).toContain(
      "El formato del correo electrónico no es válido.",
    );
    expect(screen.queryByText(/cuenta creada/i)).toBeNull();
  });
});
