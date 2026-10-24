import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { PokerRoom } from "@/features/poker/components/poker-room";

// Título propio de la pantalla de la sala de Planning Poker.
export const metadata: Metadata = {
  title: "Planning Poker | Software Metrics & Estimation",
};

// Pantalla protegida de una sesión de Planning Poker: la sala en vivo si la sesión está abierta
// (005/US1 a US7) o el resultado con sus rondas y votos si ya está cerrada (005/US8), lo que resuelve
// la sala según la respuesta del canal. Es un Server Component que solo toma los identificadores de
// la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque el token vive en
// sessionStorage, y muestra el regreso a la historia y la sala únicamente cuando hay una sesión
// vigente.
export default async function PokerSessionPage({
  params,
}: {
  params: Promise<{ projectId: string; storyId: string; sessionId: string }>;
}) {
  const { projectId, storyId, sessionId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Planning Poker</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}/backlog/${storyId}`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Volver a la historia
          </Link>
          <PokerRoom sessionId={sessionId} />
        </div>
      </SessionGuard>
    </main>
  );
}
