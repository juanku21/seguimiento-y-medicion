// Tema de la interfaz: la clase .dark en <html> decide la paleta y localStorage recuerda la elección.
const THEME_KEY = "theme";

// Script que aplica el tema antes de pintar la página, para evitar el parpadeo: usa la preferencia
// guardada o, si no hay ninguna o no se puede leer, la del sistema operativo.
export const THEME_INIT_SCRIPT = `(function(){var t=null;try{t=window.localStorage.getItem("${THEME_KEY}")}catch(e){}var d=t?t==="dark":window.matchMedia("(prefers-color-scheme: dark)").matches;document.documentElement.classList.toggle("dark",d)})();`;

// Suscriptores que se vuelven a renderizar cuando cambia el tema.
const listeners = new Set<() => void>();

// Registra un suscriptor y devuelve la función que lo da de baja, como pide useSyncExternalStore.
export function subscribeTheme(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// Indica si el documento está en modo oscuro.
export function isDarkTheme(): boolean {
  return document.documentElement.classList.contains("dark");
}

// Cambia de tema y lo recuerda. Si el navegador no permite guardar, el cambio vale para esta página.
export function toggleTheme(): void {
  const dark = document.documentElement.classList.toggle("dark");
  try {
    window.localStorage.setItem(THEME_KEY, dark ? "dark" : "light");
  } catch {
    // Sin almacenamiento disponible solo se pierde el recuerdo de la elección.
  }
  listeners.forEach((listener) => listener());
}
