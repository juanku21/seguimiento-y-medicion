import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { TakeFacilitatorButton } from "@/features/poker/components/take-facilitator-button";

// Pruebas del botón para tomar el rol de facilitador de la sala de Planning Poker (T073). Si el rol
// se puede tomar lo resuelve el backend (FR-068, FR-069) y la instantánea lo informa en
// facilitatorAvailableAt: acá se verifica qué ofrece la vista según ese momento y el reloj, y qué
// acción dispara la pulsación; el envío de POST /facilitator es de la sala (T077).
//
// Contrato que fijan estas pruebas para TakeFacilitatorButton:
// - Recibe facilitatorAvailableAt (string ISO 8601 o null) y onTake().
// - Con facilitatorAvailableAt en null (facilitador conectado) no muestra nada.
// - Con facilitatorAvailableAt vencido (igual o anterior al reloj) ofrece el botón con nombre
//   accesible exacto "Tomar el rol"; pulsarlo llama a onTake una vez.
// - Con facilitatorAvailableAt en el futuro no ofrece el botón y muestra, con role="status", cuánto
//   falta en formato m:ss (segundos redondeados hacia arriba). La cuenta se actualiza sola cada
//   segundo y, al vencer, aparece el botón sin que cambien las propiedades: el backend no envía una
//   instantánea nueva cuando se cumplen los 5 minutos.

// Instante fijo del reloj de las pruebas.
const now = new Date("2026-10-23T14:10:00Z");

// Devuelve el instante desplazado los segundos indicados respecto del reloj, en ISO 8601.
function secondsFromNow(seconds: number): string {
  return new Date(now.getTime() + seconds * 1000).toISOString();
}

// Renderiza el botón y devuelve la acción de tomar el rol observada.
function renderButton(facilitatorAvailableAt: string | null) {
  const onTake = vi.fn<() => void>();
  render(
    <TakeFacilitatorButton
      facilitatorAvailableAt={facilitatorAvailableAt}
      onTake={onTake}
    />,
  );
  return { onTake };
}

// Devuelve el botón "Tomar el rol", o null si la vista no lo ofrece.
function takeButton(): HTMLElement | null {
  return screen.queryByRole("button", { name: "Tomar el rol" });
}

// Avanza el reloj simulado dentro de act, para que React aplique las actualizaciones de la cuenta.
function advance(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

describe("005/US6 - Sobrevivir a las desconexiones y relevar al facilitador", () => {
  // Reloj simulado fijo; solo se simulan los temporizadores y la fecha, no las microtareas.
  beforeEach(() => {
    vi.useFakeTimers({
      toFake: [
        "setTimeout",
        "clearTimeout",
        "setInterval",
        "clearInterval",
        "Date",
      ],
    });
    vi.setSystemTime(now);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  // Caso de error: con el facilitador conectado no se ofrece tomar el rol ni se muestra una cuenta
  // (FR-069).
  it("Facilitador presente", () => {
    const { onTake } = renderButton(null);

    expect(takeButton()).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    expect(onTake).not.toHaveBeenCalled();
  });

  // Caso de error: con el facilitador desconectado hace dos minutos no se ofrece el botón y se
  // muestra cuánto falta para que el rol quede disponible, actualizado cada segundo (FR-069).
  it("Rol todavía no disponible", () => {
    renderButton(secondsFromNow(180));

    expect(takeButton()).toBeNull();
    expect(screen.getByRole("status").textContent).toContain("3:00");

    advance(1000);
    expect(screen.getByRole("status").textContent).toContain("2:59");
    expect(takeButton()).toBeNull();
  });

  // Caso límite: faltando menos de un segundo, la cuenta muestra 0:01 y no 0:00, y el botón todavía
  // no aparece.
  it("redondea hacia arriba el último segundo de espera", () => {
    renderButton(new Date(now.getTime() + 400).toISOString());

    expect(takeButton()).toBeNull();
    expect(screen.getByRole("status").textContent).toContain("0:01");
  });

  // Caso límite: con el facilitador desconectado hace más de 5 minutos el único participante
  // conectado ve el botón y al pulsarlo pide el rol (FR-068).
  it("Único participante toma el rol", () => {
    const { onTake } = renderButton(secondsFromNow(-120));

    const button = takeButton();
    expect(button).not.toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.click(button as HTMLElement);
    expect(onTake).toHaveBeenCalledTimes(1);
  });

  // Caso límite: el rol queda disponible justo a los 5 minutos; con facilitatorAvailableAt igual al
  // reloj el botón ya se ofrece (FR-068, RN-25).
  it("ofrece Tomar el rol justo al cumplirse los 5 minutos", () => {
    renderButton(secondsFromNow(0));

    expect(takeButton()).not.toBeNull();
  });

  // Regla de la interfaz: la cuenta llega a cero sola y el botón aparece sin una instantánea nueva,
  // porque el vencimiento de los 5 minutos no es un hecho de la sesión (FR-068).
  it("ofrece Tomar el rol al vencer la espera sin una instantánea nueva", () => {
    const { onTake } = renderButton(secondsFromNow(3));

    expect(takeButton()).toBeNull();
    advance(3000);

    const button = takeButton();
    expect(button).not.toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    fireEvent.click(button as HTMLElement);
    expect(onTake).toHaveBeenCalledTimes(1);
  });
});
