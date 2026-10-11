import type { Metadata } from "next";
import "@/styles/globals.css";

// Título y descripción por defecto de todas las pantallas de la aplicación.
export const metadata: Metadata = {
  title: "Software Metrics & Estimation",
  description:
    "Sistema de seguimiento, medición y estimación de proyectos de software.",
};

// Layout raíz: declara el idioma del documento y aplica los colores base y la tipografía a todas
// las rutas.
export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="es" className="h-full antialiased">
      <body className="flex min-h-full flex-col bg-background font-sans text-foreground">
        {children}
      </body>
    </html>
  );
}
