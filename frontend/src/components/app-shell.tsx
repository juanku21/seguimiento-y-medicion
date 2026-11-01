import { CircleUser, FolderKanban } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { Brand } from "@/components/brand";
import { ThemeToggle } from "@/components/theme-toggle";

const HEADER_LINK_CLASS_NAME =
  "inline-flex min-h-10 items-center gap-2 rounded-md px-3 text-sm font-medium text-foreground-muted transition-colors hover:bg-surface-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus";

// Estructura de las pantallas autenticadas: barra superior con la marca, el acceso a los proyectos
// y al perfil, donde se cierra la sesión, el selector de tema y el área de contenido.
export function AppShell({ children }: { children: ReactNode }) {
  return (
    <>
      <header className="sticky top-0 z-20 border-b border-border bg-surface">
        <div className="mx-auto flex h-14 max-w-[90rem] items-center justify-between gap-4 px-4 sm:px-6">
          <Brand href="/projects" />
          <div className="flex items-center gap-1">
            <Link href="/projects" className={HEADER_LINK_CLASS_NAME}>
              <FolderKanban aria-hidden="true" className="size-5" />
              Proyectos
            </Link>
            <Link href="/profile" className={HEADER_LINK_CLASS_NAME}>
              <CircleUser aria-hidden="true" className="size-5" />
              Perfil
            </Link>
            <ThemeToggle />
          </div>
        </div>
      </header>
      <div className="mx-auto flex w-full max-w-[90rem] flex-1 flex-col px-4 py-6 sm:px-6 lg:py-8">
        {children}
      </div>
    </>
  );
}
