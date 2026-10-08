import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tsconfigPaths from "vite-tsconfig-paths";

// Configuración de Vitest según la guía de Next.js: resuelve los alias de
// tsconfig, transforma JSX con React, simula el DOM con jsdom y mide la
// cobertura con el proveedor v8. passWithNoTests evita que la corrida falle
// mientras el proyecto todavía no tiene archivos de prueba.
export default defineConfig({
  plugins: [tsconfigPaths(), react()],
  test: {
    environment: "jsdom",
    passWithNoTests: true,
    coverage: {
      provider: "v8",
    },
  },
});
