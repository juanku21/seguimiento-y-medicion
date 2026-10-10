// Variables de entorno públicas del frontend, tipadas. Next.js reemplaza en el build cada
// referencia literal a process.env.NEXT_PUBLIC_*, por eso se lee con acceso estático y no
// dinámico. Si la variable no está definida se usa la cadena vacía: las rutas de la API quedan
// relativas al mismo origen en lugar de producir una URL con "undefined".
export const API_BASE_URL: string = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
