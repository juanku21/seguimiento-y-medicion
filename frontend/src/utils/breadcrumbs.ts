// Una miga de pan: la última no lleva enlace porque es la página actual.
export type Breadcrumb = {
  label: string;
  href?: string;
};

// Secciones del proyecto con su nombre, el de su elemento y el de su pantalla de alta.
const SECTIONS: Record<string, { label: string; item?: string; add?: string }> =
  {
    dashboard: { label: "Dashboard" },
    backlog: { label: "Backlog", item: "Historia", add: "Nueva historia" },
    sprints: { label: "Sprints", item: "Sprint", add: "Nuevo sprint" },
    effort: { label: "Esfuerzo" },
    defects: { label: "Defectos", item: "Defecto", add: "Nuevo defecto" },
    reports: { label: "Reportes" },
  };

// Arma las migas de una ruta de proyecto a partir de sus segmentos. El nombre del proyecto llega de
// la API y, mientras carga, se muestra uno genérico. La última miga no lleva enlace.
export function buildBreadcrumbs(
  pathname: string,
  projectName: string | null,
): Breadcrumb[] {
  const [, , projectId, section, item, sub, subItem] = pathname.split("/");
  const projectPath = `/projects/${projectId}`;
  const crumbs: Breadcrumb[] = [
    { label: "Proyectos", href: "/projects" },
    { label: projectName ?? "Proyecto", href: projectPath },
  ];

  if (section === "edit") {
    crumbs.push({ label: "Editar" });
  }
  const config = section ? SECTIONS[section] : undefined;
  if (config) {
    const sectionPath = `${projectPath}/${section}`;
    crumbs.push({ label: config.label, href: sectionPath });
    if (item === "new") {
      crumbs.push({ label: config.add ?? "Nuevo" });
    } else if (item) {
      crumbs.push({
        label: config.item ?? item,
        href: `${sectionPath}/${item}`,
      });
    }
    if (sub === "edit") {
      crumbs.push({ label: "Editar" });
    } else if (sub === "poker" && subItem) {
      crumbs.push({ label: "Planning Poker" });
    }
  }

  const last = crumbs[crumbs.length - 1];
  crumbs[crumbs.length - 1] = { label: last.label };
  return crumbs;
}
