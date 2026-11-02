import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { ProjectNav } from "@/components/project-nav";
import { buildBreadcrumbs } from "@/utils/breadcrumbs";

// Ruta actual que devuelve el doble de usePathname.
const location = vi.hoisted(() => ({ pathname: "/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => location.pathname,
}));

// Mejora de UX sin spec asociada: la convención NNN/USn no aplica, se justifica en el PR.
describe("UI - Navegación del proyecto", () => {
  beforeEach(() => {
    location.pathname = "/projects/p1/backlog/h1";
  });

  afterEach(cleanup);

  it("ofrece todas las secciones del proyecto", () => {
    render(<ProjectNav projectId="p1" />);

    const nav = screen.getByRole("navigation", {
      name: "Navegación del proyecto",
    });
    const hrefs = within(nav)
      .getAllByRole("link")
      .map((link) => link.getAttribute("href"));
    expect(hrefs).toEqual([
      "/projects/p1",
      "/projects/p1/dashboard",
      "/projects/p1/backlog",
      "/projects/p1/sprints",
      "/projects/p1/effort",
      "/projects/p1/defects",
      "/projects/p1/reports",
    ]);
  });

  it("marca la sección actual aunque la ruta sea más profunda", () => {
    render(<ProjectNav projectId="p1" />);

    expect(
      screen
        .getByRole("link", { name: "Backlog" })
        .getAttribute("aria-current"),
    ).toBe("page");
    expect(
      screen
        .getByRole("link", { name: "Resumen" })
        .getAttribute("aria-current"),
    ).toBeNull();
  });

  it("marca el resumen solo en la portada del proyecto", () => {
    location.pathname = "/projects/p1";

    render(<ProjectNav projectId="p1" />);

    expect(
      screen
        .getByRole("link", { name: "Resumen" })
        .getAttribute("aria-current"),
    ).toBe("page");
  });
});

describe("UI - Migas de pan", () => {
  afterEach(cleanup);

  it("arma la ruta con el nombre real del proyecto", () => {
    expect(
      buildBreadcrumbs("/projects/p1/backlog/h1/edit", "Plataforma"),
    ).toEqual([
      { label: "Proyectos", href: "/projects" },
      { label: "Plataforma", href: "/projects/p1" },
      { label: "Backlog", href: "/projects/p1/backlog" },
      { label: "Historia", href: "/projects/p1/backlog/h1" },
      { label: "Editar" },
    ]);
  });

  it("nombra las pantallas de alta y la sala de estimación", () => {
    expect(
      buildBreadcrumbs("/projects/p1/sprints/new", "Plataforma").at(-1),
    ).toEqual({ label: "Nuevo sprint" });
    expect(
      buildBreadcrumbs("/projects/p1/backlog/h1/poker/s1", "Plataforma").at(-1),
    ).toEqual({ label: "Planning Poker" });
  });

  it("muestra un nombre genérico mientras carga el proyecto", () => {
    expect(buildBreadcrumbs("/projects/p1", null)).toEqual([
      { label: "Proyectos", href: "/projects" },
      { label: "Proyecto" },
    ]);
  });

  it("marca la última miga como la página actual, sin enlace", () => {
    location.pathname = "/projects/p1/dashboard";

    render(<Breadcrumbs projectName="Plataforma" />);

    const nav = screen.getByRole("navigation", { name: "Migas de pan" });
    expect(within(nav).getAllByRole("link")).toHaveLength(2);
    expect(
      within(nav).getByText("Dashboard").getAttribute("aria-current"),
    ).toBe("page");
  });
});
