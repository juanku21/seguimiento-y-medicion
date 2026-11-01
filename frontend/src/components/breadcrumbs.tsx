"use client";

import { ChevronRight } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { buildBreadcrumbs } from "@/utils/breadcrumbs";

// Migas de pan de las pantallas de un proyecto: muestran dónde se está y dan el camino de regreso.
export function Breadcrumbs({ projectName }: { projectName: string | null }) {
  const crumbs = buildBreadcrumbs(usePathname(), projectName);
  return (
    <nav aria-label="Migas de pan">
      <ol className="flex flex-wrap items-center gap-1 text-sm text-foreground-muted">
        {crumbs.map((crumb, index) => (
          <li key={crumb.label} className="flex items-center gap-1">
            {index > 0 && (
              <ChevronRight aria-hidden="true" className="size-4 shrink-0" />
            )}
            {crumb.href ? (
              <Link
                href={crumb.href}
                className="max-w-56 truncate rounded-sm hover:text-foreground hover:underline focus-visible:outline-2 focus-visible:outline-focus"
              >
                {crumb.label}
              </Link>
            ) : (
              <span
                aria-current="page"
                className="max-w-56 truncate font-medium text-foreground"
              >
                {crumb.label}
              </span>
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}
