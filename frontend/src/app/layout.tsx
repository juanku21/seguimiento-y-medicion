import type { Metadata } from "next";
import localFont from "next/font/local";
import { THEME_INIT_SCRIPT } from "@/lib/theme";
import { SessionProvider } from "@/providers/session-provider";
import "@/styles/globals.css";

// Tipografía de toda la aplicación, autoalojada en el repositorio (licencia OFL en styles/fonts) para
// no depender de la red al compilar, y expuesta al token --font-sans.
const plexSans = localFont({
  src: [
    { path: "../styles/fonts/ibm-plex-sans-400.woff2", weight: "400" },
    { path: "../styles/fonts/ibm-plex-sans-500.woff2", weight: "500" },
    { path: "../styles/fonts/ibm-plex-sans-600.woff2", weight: "600" },
    { path: "../styles/fonts/ibm-plex-sans-700.woff2", weight: "700" },
  ],
  variable: "--font-plex-sans",
});

// Título y descripción por defecto de todas las pantallas de la aplicación.
export const metadata: Metadata = {
  title: "Software Metrics & Estimation",
  description:
    "Sistema de seguimiento, medición y estimación de proyectos de software.",
};

// Layout raíz: declara el idioma del documento, aplica el tema antes de pintar, los colores base y
// la tipografía a todas las rutas y envuelve las pantallas en el proveedor de sesión. El script
// cambia la clase de <html> antes de hidratar, por eso se suprime ese aviso de hidratación.
export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="es"
      className={`h-full antialiased ${plexSans.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
      </head>
      <body className="flex min-h-full flex-col bg-background font-sans text-foreground">
        <SessionProvider>{children}</SessionProvider>
      </body>
    </html>
  );
}
