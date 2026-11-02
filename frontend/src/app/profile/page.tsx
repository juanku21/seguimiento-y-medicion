import type { Metadata } from "next";
import { LogoutButton } from "@/features/auth/components/logout-button";
import { ProfileCard } from "@/features/auth/components/profile-card";

// Título propio de la pantalla de perfil.
export const metadata: Metadata = {
  title: "Mi perfil | Software Metrics & Estimation",
};

// Pantalla protegida del propio perfil (US4). Es un Server Component que solo encuadra la tarjeta:
// la tarjeta corre en el cliente, aplica la guardia de sesión (T061) y pide el perfil con el token
// de la pestaña, que vive en sessionStorage y no está disponible en el servidor. Debajo ofrece el
// botón de cierre de sesión (US5), que también corre
// en el cliente.
export default function ProfilePage() {
  return (
    <main className="flex w-full max-w-2xl flex-1 flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Mi perfil</h1>
      <ProfileCard />
      <LogoutButton />
    </main>
  );
}
