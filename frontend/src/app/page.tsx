import { Bug, CalendarRange, Timer, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { buttonClassName } from "@/components/button";
import { PublicShell } from "@/components/public-shell";

// Lo que la persona puede hacer con el producto.
const CAPABILITIES: { icon: LucideIcon; title: string; text: string }[] = [
  {
    icon: CalendarRange,
    title: "Planificá con datos",
    text: "Estimá historias con Planning Poker y comprometé sprints según la velocidad real del equipo.",
  },
  {
    icon: Timer,
    title: "Compará lo estimado con lo real",
    text: "Registrá el esfuerzo por historia y detectá las desviaciones mientras todavía podés corregirlas.",
  },
  {
    icon: Bug,
    title: "Medí la calidad",
    text: "Seguí los defectos por severidad y por sprint, y generá reportes para compartir.",
  },
];

// Portada pública: presenta el producto y lleva a crear una cuenta o a iniciar sesión.
export default function HomePage() {
  return (
    <PublicShell>
      <main className="mx-auto grid w-full max-w-6xl flex-1 content-center gap-12 px-6 py-12 lg:grid-cols-2 lg:gap-16">
        <div className="flex flex-col justify-center gap-6">
          <h1 className="text-4xl font-semibold tracking-tight text-balance sm:text-5xl">
            Software Metrics &amp; Estimation
          </h1>
          <p className="max-w-xl text-lg leading-8 text-foreground-muted">
            Registrá el esfuerzo de tu equipo, seguí el avance de los sprints y
            medí la calidad de tus proyectos de software.
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Link href="/register" className={buttonClassName("primary")}>
              Crear cuenta
            </Link>
            <Link href="/login" className={buttonClassName("secondary")}>
              Iniciar sesión
            </Link>
          </div>
        </div>
        <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface">
          {CAPABILITIES.map(({ icon: Icon, title, text }) => (
            <li key={title} className="flex gap-4 p-6">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary-soft text-primary">
                <Icon aria-hidden="true" className="size-5" />
              </span>
              <div className="flex flex-col gap-1">
                <h2 className="font-semibold">{title}</h2>
                <p className="text-sm leading-6 text-foreground-muted">
                  {text}
                </p>
              </div>
            </li>
          ))}
        </ul>
      </main>
    </PublicShell>
  );
}
