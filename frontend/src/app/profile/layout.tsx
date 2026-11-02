import { AppShell } from "@/components/app-shell";

// La pantalla de perfil comparte la barra superior de la aplicación.
export default function ProfileLayout({ children }: LayoutProps<"/profile">) {
  return <AppShell>{children}</AppShell>;
}
