import { AppShell } from "@/components/app-shell";

// Las pantallas de proyectos comparten la barra superior de la aplicación.
export default function ProjectsLayout({ children }: LayoutProps<"/projects">) {
  return <AppShell>{children}</AppShell>;
}
