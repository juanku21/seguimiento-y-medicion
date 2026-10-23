import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { ParticipantList } from "@/features/poker/components/participant-list";
import type { EstimationParticipant, UserRef } from "@/types/poker";

// Pruebas de la lista de participantes de la sala de Planning Poker (T017). La lista recibe los
// participantes tal como llegan en cada instantánea del canal de eventos: quién está conectado y
// quién es el facilitador lo resuelve el backend (Presence y ViewFor). Acá se verifica qué muestra
// la vista con cada instantánea, sin recargar: cada prueba vuelve a renderizar con la instantánea
// nueva, como hace la sala al recibir un evento state.
//
// Contrato que fijan estas pruebas para ParticipantList:
// - Recibe participants (EstimationParticipant[]).
// - Muestra una lista con nombre accesible "Participantes" y un elemento por participante, en el
//   orden recibido, con su nombre completo.
// - Cada elemento muestra su estado de conexión con el texto exacto "Conectado" o "Desconectado":
//   quien se desconectó sigue en la lista, marcado como desconectado.
// - Solo el elemento del facilitador muestra la marca con el texto exacto "Facilitador".

// Integrantes del proyecto que participan de la sesión.
const ana: UserRef = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
};
const bruno: UserRef = {
  id: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
  fullName: "Bruno Gómez",
};
const carla: UserRef = {
  id: "5a9e3c71-4f28-4b06-8d1e-6c2b7f0a9e13",
  fullName: "Carla Ruiz",
};

// Arma un participante de la ronda vigente, todavía sin voto.
function participant(
  user: UserRef,
  connected: boolean,
  isFacilitator = false,
): EstimationParticipant {
  return { user, connected, isFacilitator, hasVoted: false };
}

// Devuelve los elementos de la lista de participantes, en el orden mostrado.
function shownParticipants(): HTMLElement[] {
  const list = screen.getByRole("list", { name: "Participantes" });
  return within(list).getAllByRole("listitem");
}

// Verifica el nombre, el estado de conexión y la marca de facilitador de un elemento de la lista.
function expectParticipant(
  item: HTMLElement,
  user: UserRef,
  connected: boolean,
  isFacilitator: boolean,
) {
  const scope = within(item);
  expect(item.textContent, user.fullName).toContain(user.fullName);
  expect(
    scope.queryByText(connected ? "Conectado" : "Desconectado"),
    user.fullName,
  ).not.toBeNull();
  expect(
    scope.queryByText(connected ? "Desconectado" : "Conectado"),
    user.fullName,
  ).toBeNull();
  if (isFacilitator) {
    expect(scope.queryByText("Facilitador"), user.fullName).not.toBeNull();
  } else {
    expect(scope.queryByText("Facilitador"), user.fullName).toBeNull();
  }
}

describe("005/US1 - Abrir una sesión y reunir al equipo", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: recién abierta la sesión, quien la abrió figura como facilitador y como primer
  // participante conectado (FR-002).
  it("Apertura de sesión", () => {
    render(<ParticipantList participants={[participant(ana, true, true)]} />);

    const items = shownParticipants();
    expect(items).toHaveLength(1);
    expectParticipant(items[0], ana, true, true);
  });

  // Caso normal: con la sesión abierta por Ana, se unen Bruno y Carla; cada instantánea nueva
  // actualiza la lista sin recargar y al final los tres figuran conectados, con la marca de
  // facilitador solo en Ana (FR-011, FR-013).
  it("Incorporación de dos integrantes", () => {
    const { rerender } = render(
      <ParticipantList participants={[participant(ana, true, true)]} />,
    );
    expect(shownParticipants()).toHaveLength(1);

    rerender(
      <ParticipantList
        participants={[participant(ana, true, true), participant(bruno, true)]}
      />,
    );
    expect(shownParticipants()).toHaveLength(2);

    rerender(
      <ParticipantList
        participants={[
          participant(ana, true, true),
          participant(bruno, true),
          participant(carla, true),
        ]}
      />,
    );
    const items = shownParticipants();
    expect(items).toHaveLength(3);
    expectParticipant(items[0], ana, true, true);
    expectParticipant(items[1], bruno, true, false);
    expectParticipant(items[2], carla, true, false);
  });

  // Caso alternativo: de tres conectados, Carla abandona la sesión; al llegar la instantánea los
  // otros dos la ven como desconectada, sin recargar, y ella sigue en la lista (FR-014).
  it("Abandono de un participante", () => {
    const { rerender } = render(
      <ParticipantList
        participants={[
          participant(ana, true, true),
          participant(bruno, true),
          participant(carla, true),
        ]}
      />,
    );
    expectParticipant(shownParticipants()[2], carla, true, false);

    rerender(
      <ParticipantList
        participants={[
          participant(ana, true, true),
          participant(bruno, true),
          participant(carla, false),
        ]}
      />,
    );

    const items = shownParticipants();
    expect(items).toHaveLength(3);
    expectParticipant(items[0], ana, true, true);
    expectParticipant(items[1], bruno, true, false);
    expectParticipant(items[2], carla, false, false);
  });

  // Caso límite: nadie más se une y el facilitador es el único participante; la lista lo muestra
  // solo, conectado y con su marca, sin presentarlo como un error.
  it("Facilitador solo", () => {
    render(<ParticipantList participants={[participant(bruno, true, true)]} />);

    const items = shownParticipants();
    expect(items).toHaveLength(1);
    expectParticipant(items[0], bruno, true, true);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Regla de la interfaz: la marca sigue a quien tiene el rol según la instantánea, no a quien
  // figura primero en la lista.
  it("muestra la marca de facilitador solo en quien tiene el rol", () => {
    render(
      <ParticipantList
        participants={[
          participant(ana, false),
          participant(bruno, true),
          participant(carla, true, true),
        ]}
      />,
    );

    const items = shownParticipants();
    expectParticipant(items[0], ana, false, false);
    expectParticipant(items[1], bruno, true, false);
    expectParticipant(items[2], carla, true, true);
  });
});
