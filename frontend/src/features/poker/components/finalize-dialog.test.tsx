import { afterEach, describe, expect, it, vi, type Mock } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { FinalizeDialog } from "@/features/poker/components/finalize-dialog";
import type { ScaleStoryPoints } from "@/types/poker";

// Pruebas del diálogo de registro de la estimación acordada (T064). Que el valor pertenezca a la
// escala, que haya una ronda revelada y que quien registra sea el facilitador lo resuelve el backend
// (FR-050, RN-18); la sala muestra el diálogo solo al facilitador y envía POST /finalize (T069). Acá
// se verifica qué valores ofrece el diálogo y qué valor entrega al registrar.
//
// Contrato que fijan estas pruebas para FinalizeDialog:
// - Recibe onFinalize(storyPoints: ScaleStoryPoints).
// - El valor se elige en un selector con la etiqueta "Estimación acordada", que ofrece solo los
//   diez valores numéricos de la escala, en orden, cada uno con el número como texto y como valor:
//   sin la carta "?", sin "Sin estimar" y sin opción vacía. No hay campo de texto ni numérico libre.
// - El botón "Registrar estimación" llama una vez a onFinalize con el valor elegido como número.

type FinalizeMock = Mock<(storyPoints: ScaleStoryPoints) => void>;

// Valores de la escala que ofrece el selector, en orden (RC-04, FR-049).
const scaleValues = ["0", "1", "2", "3", "5", "8", "13", "20", "40", "100"];

// Monta el diálogo y devuelve el doble de onFinalize.
function renderDialog(): FinalizeMock {
  const onFinalize: FinalizeMock = vi.fn();
  render(<FinalizeDialog onFinalize={onFinalize} />);
  return onFinalize;
}

// Devuelve el selector de la estimación acordada.
function agreedSelect(): HTMLSelectElement {
  const select = screen.getByLabelText("Estimación acordada");
  if (!(select instanceof HTMLSelectElement)) {
    throw new Error("Estimación acordada debe ser un selector");
  }
  return select;
}

// Elige un valor del selector y registra la estimación: los dos pasos del flujo de registro.
function finalize(value: string) {
  fireEvent.change(agreedSelect(), { target: { value } });
  fireEvent.click(screen.getByRole("button", { name: "Registrar estimación" }));
}

describe("005/US5 - Registrar la estimación acordada", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: con consenso en 8, el facilitador elige 8 y el diálogo entrega el número 8 para
  // registrarlo (FR-049).
  it("Registro con consenso", () => {
    const onFinalize = renderDialog();

    finalize("8");

    expect(onFinalize).toHaveBeenCalledTimes(1);
    expect(onFinalize).toHaveBeenCalledWith(8);
  });

  // Caso alternativo: el facilitador registra 13 aunque nadie lo votó; el diálogo no limita los
  // valores a los votos de la ronda (RN-18).
  it("Valor no votado", () => {
    const onFinalize = renderDialog();

    finalize("13");

    expect(onFinalize).toHaveBeenCalledTimes(1);
    expect(onFinalize).toHaveBeenCalledWith(13);
  });

  // Caso límite: aunque todos votaron "?", el facilitador registra 40; el valor lo decide él y no
  // se calcula a partir de los votos.
  it("Registro tras todos interrogación", () => {
    const onFinalize = renderDialog();

    finalize("40");

    expect(onFinalize).toHaveBeenCalledTimes(1);
    expect(onFinalize).toHaveBeenCalledWith(40);
  });

  // Caso de error: el diálogo no permite elegir 7, un decimal, la carta "?" ni "sin estimar":
  // ofrece solo los diez valores numéricos de la escala y ningún campo libre (FR-050, RN-18).
  it("Valor inválido", () => {
    renderDialog();

    const options = Array.from(agreedSelect().options);
    expect(options.map((option) => option.textContent)).toEqual(scaleValues);
    expect(options.map((option) => option.value)).toEqual(scaleValues);
    expect(
      options.some((option) =>
        /sin estimar|\?/i.test(option.textContent ?? ""),
      ),
    ).toBe(false);
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("spinbutton")).toBeNull();
  });

  // Regla de la interfaz: el 0 es un valor acordado válido y se entrega como el número 0, distinto
  // de "sin estimar" (RC-05).
  it("registra 0 como valor numérico", () => {
    const onFinalize = renderDialog();

    finalize("0");

    expect(onFinalize).toHaveBeenCalledTimes(1);
    expect(onFinalize).toHaveBeenCalledWith(0);
  });
});
