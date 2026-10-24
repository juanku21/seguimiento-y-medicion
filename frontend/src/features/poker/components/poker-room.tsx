"use client";

import { useState, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { getOwnProfile } from "@/features/auth/api";
import {
  castEstimationVote,
  finalizeEstimationSession,
  revealEstimationRound,
  startEstimationRound,
  takeEstimationFacilitator,
} from "@/features/poker/api";
import { CardDeck } from "@/features/poker/components/card-deck";
import { FacilitatorActions } from "@/features/poker/components/facilitator-actions";
import { FinalizeDialog } from "@/features/poker/components/finalize-dialog";
import { ParticipantList } from "@/features/poker/components/participant-list";
import { RoundHistory } from "@/features/poker/components/round-history";
import { RoundResult } from "@/features/poker/components/round-result";
import { SessionClosedNotice } from "@/features/poker/components/session-closed-notice";
import { TakeFacilitatorButton } from "@/features/poker/components/take-facilitator-button";
import { useEstimationStream } from "@/features/poker/use-estimation-stream";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import type { Card, ScaleStoryPoints } from "@/types/poker";

// Propiedades de la sala en vivo: la sesión de estimación tomada de la ruta.
export type PokerRoomProps = {
  sessionId: string;
};

// Sala en vivo de una sesión de Planning Poker (005/US1). Abre el canal de la sesión, que une a
// quien entra como participante conectado, y con cada instantánea muestra el título, la descripción
// y los criterios de aceptación vigentes de la historia y la lista de participantes, sin recargar
// (FR-011, FR-013, FR-075). Si el canal se rechaza (sesión inexistente o ajena) muestra solo el
// mensaje recibido. El mazo de la ronda vigente envía el voto propio (005/US2); la sesión resultante
// llega por el canal y, si el voto se rechaza, se muestra el motivo recibido. El facilitador, que se
// reconoce comparando el perfil propio con el de la instantánea, revela la ronda (005/US3); la ronda
// revelada muestra los votos y su resultado, y un rechazo de la revelación muestra el motivo recibido.
// Con la ronda revelada el facilitador inicia una ronda nueva (005/US4) y las rondas anteriores se
// muestran en el historial; un rechazo de la ronda nueva muestra el motivo recibido. Con alguna ronda
// revelada el facilitador registra la estimación acordada (005/US5); un rechazo muestra el motivo
// recibido y, cuando la instantánea llega cerrada, la sala muestra el aviso de cierre con su
// resultado en lugar del mazo y de las acciones. Con el facilitador desconectado la sala muestra
// cuánto falta para que el rol quede disponible y, al vencer, ofrece tomarlo (005/US6); un rechazo
// muestra el motivo recibido.
export function PokerRoom({ sessionId }: PokerRoomProps): ReactElement {
  const { session, error } = useEstimationStream(sessionId);
  const profile = useAuthorizedQuery(getOwnProfile);
  const [voteError, setVoteError] = useState<string | null>(null);
  const [roundError, setRoundError] = useState<string | null>(null);
  const [finalizeError, setFinalizeError] = useState<string | null>(null);
  const [takeError, setTakeError] = useState<string | null>(null);

  // Envía la carta jugada; ante un rechazo muestra el detalle del campo si lo hay o el mensaje
  // general, y ante un voto aceptado limpia el aviso anterior.
  const handleVote = async (card: Card) => {
    const result = await castEstimationVote(sessionId, card);
    setVoteError(
      result.ok
        ? null
        : (result.error.fields[0]?.message ?? result.error.message),
    );
  };

  // Pide la revelación manual; ante un rechazo muestra el mensaje recibido y ante una revelación
  // aceptada limpia el aviso anterior. La ronda revelada llega por el canal.
  const handleReveal = async () => {
    const result = await revealEstimationRound(sessionId);
    setRoundError(result.ok ? null : result.error.message);
  };

  // Pide la ronda nueva; ante un rechazo muestra el mensaje recibido y ante una ronda aceptada limpia
  // el aviso anterior. La ronda nueva llega por el canal.
  const handleNewRound = async () => {
    const result = await startEstimationRound(sessionId);
    setRoundError(result.ok ? null : result.error.message);
  };

  // Registra el valor acordado; ante un rechazo muestra el detalle del campo si lo hay o el mensaje
  // general, y ante un registro aceptado limpia el aviso anterior. La sesión cerrada llega por el
  // canal.
  const handleFinalize = async (storyPoints: ScaleStoryPoints) => {
    const result = await finalizeEstimationSession(sessionId, storyPoints);
    setFinalizeError(
      result.ok
        ? null
        : (result.error.fields[0]?.message ?? result.error.message),
    );
  };

  // Pide el rol de facilitador; ante un rechazo muestra el mensaje recibido y ante un pedido aceptado
  // limpia el aviso anterior. La sesión con el nuevo facilitador llega por el canal.
  const handleTakeFacilitator = async () => {
    const result = await takeEstimationFacilitator(sessionId);
    setTakeError(result.ok ? null : result.error.message);
  };

  if (error !== null) {
    return <Alert>{error.message}</Alert>;
  }
  if (session === null) {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Conectando con la sesión…
      </p>
    );
  }

  const { story } = session;
  const isOpen = session.status === "open";
  const currentRound = session.rounds.at(-1);
  const isFacilitator =
    profile.status === "loaded" && profile.data.id === session.facilitator.id;
  const hasRevealedRound = session.rounds.some(
    (round) => round.status === "revealed",
  );
  return (
    <div className="flex flex-col gap-6">
      {/* Historia que se estima con su texto vigente; sin criterios la lista queda vacía y se
          avisa con un texto aparte. */}
      <article className="flex flex-col gap-4 rounded-md border border-zinc-300 px-5 py-5 dark:border-zinc-700">
        <h2 className="text-xl font-semibold break-words">{story.title}</h2>
        {story.description !== null && (
          <p className="text-sm break-words whitespace-pre-line">
            {story.description}
          </p>
        )}
        <section className="flex flex-col gap-2">
          <h3 className="text-base font-medium">Criterios de aceptación</h3>
          <ul
            aria-label="Criterios de aceptación"
            className="flex list-disc flex-col gap-1 pl-5 text-sm"
          >
            {story.acceptanceCriteria.map((criterion, index) => (
              <li key={index} className="break-words">
                {criterion}
              </li>
            ))}
          </ul>
          {story.acceptanceCriteria.length === 0 && (
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              La historia no tiene criterios de aceptación.
            </p>
          )}
        </section>
      </article>

      {/* Mazo de la ronda vigente con la carta propia resaltada y el motivo de un voto rechazado. */}
      {isOpen && currentRound !== undefined && (
        <section className="flex flex-col gap-3">
          <h2 className="text-lg font-semibold">Tu voto</h2>
          {voteError !== null && <Alert>{voteError}</Alert>}
          <CardDeck
            cards={session.cards}
            myVote={session.myVote}
            roundStatus={currentRound.status}
            onVote={(card) => void handleVote(card)}
          />
        </section>
      )}

      {/* Acciones del facilitador sobre la ronda vigente con el motivo de un rechazo, mientras la
          sesión está abierta, y el resultado cuando la ronda ya está revelada. */}
      {currentRound !== undefined && (
        <section className="flex flex-col gap-3">
          <h2 className="text-lg font-semibold">Ronda {currentRound.number}</h2>
          {roundError !== null && <Alert>{roundError}</Alert>}
          <FacilitatorActions
            isFacilitator={isOpen && isFacilitator}
            roundStatus={currentRound.status}
            onReveal={() => void handleReveal()}
            onNewRound={() => void handleNewRound()}
          />
          {currentRound.status === "revealed" && (
            <RoundResult round={currentRound} />
          )}
        </section>
      )}

      {/* Registro de la estimación acordada, solo para el facilitador y con alguna ronda revelada,
          con el motivo de un rechazo. */}
      {isOpen && isFacilitator && hasRevealedRound && (
        <section className="flex flex-col gap-3">
          <h2 className="text-lg font-semibold">Registrar estimación</h2>
          {finalizeError !== null && <Alert>{finalizeError}</Alert>}
          <FinalizeDialog
            onFinalize={(storyPoints) => void handleFinalize(storyPoints)}
          />
        </section>
      )}

      {/* Aviso de la sesión cerrada con su estado y el valor acordado. */}
      {session.status !== "open" && (
        <SessionClosedNotice
          status={session.status}
          agreedStoryPoints={session.agreedStoryPoints}
          cancelReason={session.cancelReason}
        />
      )}

      {/* Rondas anteriores a la vigente con sus votos y su resultado. */}
      <RoundHistory rounds={session.rounds} />

      {/* Participantes de la sesión con su conexión, la marca de facilitador y la de voto, y, con la
          sesión abierta, la toma del rol si el facilitador está desconectado. */}
      <section className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold">Participantes</h2>
        <ParticipantList participants={session.participants} />
        {isOpen && (
          <>
            {takeError !== null && <Alert>{takeError}</Alert>}
            <TakeFacilitatorButton
              facilitatorAvailableAt={session.facilitatorAvailableAt}
              onTake={() => void handleTakeFacilitator()}
            />
          </>
        )}
      </section>
    </div>
  );
}
