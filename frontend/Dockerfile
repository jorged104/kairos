# Solo desarrollo: sirve la app con el servidor de Vite.
FROM node:24-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev"]
