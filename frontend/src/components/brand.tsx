import { Gauge } from "lucide-react";
import Link from "next/link";

// Marca del producto: ícono de medición y nombre, con enlace al destino indicado.
export function Brand({ href }: { href: string }) {
  return (
    <Link
      href={href}
      aria-label="Software Metrics & Estimation"
      className="flex min-h-10 items-center gap-2 rounded-md font-semibold tracking-tight text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
    >
      <span className="flex size-8 items-center justify-center rounded-md bg-primary text-on-primary">
        <Gauge aria-hidden="true" className="size-5" />
      </span>
      <span className="hidden md:inline">
        Software Metrics &amp; Estimation
      </span>
    </Link>
  );
}
