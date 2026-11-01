"use client";

import {
  Bug,
  CalendarRange,
  ClipboardList,
  FileText,
  FolderKanban,
  LayoutDashboard,
  Timer,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

// Secciones del proyecto, en el orden en que se recorren.
const SECTIONS: { label: string; segment: string; icon: LucideIcon }[] = [
  { label: "Resumen", segment: "", icon: FolderKanban },
  { label: "Dashboard", segment: "/dashboard", icon: LayoutDashboard },
  { label: "Backlog", segment: "/backlog", icon: ClipboardList },
  { label: "Sprints", segment: "/sprints", icon: CalendarRange },
  { label: "Esfuerzo", segment: "/effort", icon: Timer },
  { label: "Defectos", segment: "/defects", icon: Bug },
  { label: "Reportes", segment: "/reports", icon: FileText },
];

// Navegación del proyecto: columna lateral en escritorio y pestañas desplazables en móvil. Una
// sección queda marcada también en sus pantallas internas; el resumen, solo en la portada.
export function ProjectNav({ projectId }: { projectId: string }) {
  const pathname = usePathname();
  const basePath = `/projects/${projectId}`;

  return (
    <nav
      aria-label="Navegación del proyecto"
      className="-mx-4 overflow-x-auto px-4 sm:-mx-6 sm:px-6 lg:mx-0 lg:overflow-visible lg:px-0"
    >
      <ul className="flex gap-1 lg:flex-col">
        {SECTIONS.map(({ label, segment, icon: Icon }) => {
          const href = `${basePath}${segment}`;
          const isCurrent =
            segment === ""
              ? pathname === basePath
              : pathname === href || pathname.startsWith(`${href}/`);
          return (
            <li key={label} className="shrink-0">
              <Link
                href={href}
                aria-current={isCurrent ? "page" : undefined}
                className="flex min-h-11 items-center gap-2.5 rounded-md px-3 text-sm font-medium text-foreground-muted transition-colors hover:bg-surface-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus aria-[current=page]:bg-primary-soft aria-[current=page]:text-primary lg:min-h-10"
              >
                <Icon aria-hidden="true" className="size-4 shrink-0" />
                {label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
