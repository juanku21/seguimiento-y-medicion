import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeToggle } from "@/components/theme-toggle";
import { THEME_INIT_SCRIPT } from "@/lib/theme";

// Simula la preferencia de color del sistema operativo.
function stubSystemPreference(prefersDark: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn<(query: string) => Partial<MediaQueryList>>((query) => ({
      matches: prefersDark && query.includes("dark"),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
}

// Indica si el documento está en modo oscuro.
function isDark(): boolean {
  return document.documentElement.classList.contains("dark");
}

// Ejecuta el script de inicio igual que lo hace el navegador antes de pintar.
function runInitScript() {
  new Function(THEME_INIT_SCRIPT)();
}

// Mejora de UX sin spec asociada: la convención NNN/USn no aplica, se justifica en el PR.
describe("UI - Selector de tema", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.classList.remove("dark");
    stubSystemPreference(false);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("en modo claro muestra la luna y ofrece pasar a oscuro", () => {
    render(<ThemeToggle />);

    const button = screen.getByRole("button", { name: "Activar modo oscuro" });
    expect(button.querySelector(".lucide-moon")).not.toBeNull();
  });

  it("en modo oscuro muestra el sol y ofrece pasar a claro", () => {
    document.documentElement.classList.add("dark");

    render(<ThemeToggle />);

    const button = screen.getByRole("button", { name: "Activar modo claro" });
    expect(button.querySelector(".lucide-sun")).not.toBeNull();
  });

  it("al pulsarlo cambia el tema, lo recuerda y actualiza el botón", () => {
    render(<ThemeToggle />);

    fireEvent.click(
      screen.getByRole("button", { name: "Activar modo oscuro" }),
    );

    expect(isDark()).toBe(true);
    expect(window.localStorage.getItem("theme")).toBe("dark");
    fireEvent.click(screen.getByRole("button", { name: "Activar modo claro" }));
    expect(isDark()).toBe(false);
    expect(window.localStorage.getItem("theme")).toBe("light");
  });

  it("sigue cambiando el tema si no se puede guardar la preferencia", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("almacenamiento no disponible");
    });
    render(<ThemeToggle />);

    fireEvent.click(
      screen.getByRole("button", { name: "Activar modo oscuro" }),
    );

    expect(isDark()).toBe(true);
  });

  it("sin preferencia guardada, el inicio sigue al sistema operativo", () => {
    stubSystemPreference(true);

    runInitScript();

    expect(isDark()).toBe(true);
  });

  it("la preferencia guardada prevalece sobre el sistema operativo", () => {
    stubSystemPreference(true);
    window.localStorage.setItem("theme", "light");

    runInitScript();

    expect(isDark()).toBe(false);
  });
});
