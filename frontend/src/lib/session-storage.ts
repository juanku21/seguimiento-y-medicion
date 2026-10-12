// Persistencia del token de sesión en sessionStorage (research.md, sección 9): el token sobrevive al
// refresco de la pestaña pero no al cierre del navegador.
const TOKEN_KEY = "sessionToken";

// Indica si hay sessionStorage disponible. Next.js también prerenderiza los Client Components en el
// servidor, donde no existe window, y ahí no hay sesión que leer ni guardar.
function hasSessionStorage(): boolean {
  return typeof window !== "undefined";
}

// Devuelve el token guardado en la pestaña, o null si no hay sesión o si se ejecuta en el servidor.
export function readToken(): string | null {
  if (!hasSessionStorage()) {
    return null;
  }
  return window.sessionStorage.getItem(TOKEN_KEY);
}

// Guarda el token recibido al iniciar sesión; en el servidor no hace nada.
export function writeToken(token: string): void {
  if (!hasSessionStorage()) {
    return;
  }
  window.sessionStorage.setItem(TOKEN_KEY, token);
}
