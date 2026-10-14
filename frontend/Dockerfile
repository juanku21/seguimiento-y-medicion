# Imagen de la aplicación Next.js en dos etapas: la primera instala todas las dependencias y
# compila; la segunda solo contiene las dependencias de producción y el resultado de la compilación.
# Las versiones de las imágenes están fijadas (Principio IV).
# Construcción, desde frontend/ (NEXT_PUBLIC_API_BASE_URL se incrusta en el bundle al compilar):
#   docker build --build-arg NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 -t smye-frontend .
# Ejecución:
#   docker run -p 3000:3000 smye-frontend

# Etapa de compilación: Node.js 24.21.0, el runtime del proyecto (research.md).
FROM node:24.21.0-alpine3.24 AS build
WORKDIR /app

# Se instalan las dependencias exactas del lockfile antes de copiar el código, para aprovechar la
# caché de capas.
COPY package.json package-lock.json ./
RUN npm ci

# Las variables NEXT_PUBLIC_ se resuelven durante next build y quedan fijas en el bundle del
# navegador, por eso la URL de la API llega como argumento de construcción y no en tiempo de
# ejecución.
ARG NEXT_PUBLIC_API_BASE_URL
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
ENV NEXT_TELEMETRY_DISABLED=1

# Se copia el código y se compila la aplicación.
COPY . .
RUN npm run build

# Etapa de ejecución: misma imagen base, solo dependencias de producción y el usuario sin
# privilegios que trae la imagen oficial.
FROM node:24.21.0-alpine3.24
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Dependencias de producción del lockfile, sin las de pruebas ni de formateo.
COPY package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force

# Resultado de la compilación, recursos estáticos y configuración que lee next start.
COPY --from=build --chown=node:node /app/.next ./.next
COPY --from=build /app/public ./public
COPY --from=build /app/next.config.ts ./next.config.ts
USER node

# El puerto se puede cambiar con la variable PORT, que debe coincidir con el puerto publicado.
EXPOSE 3000
CMD ["npm", "start"]
