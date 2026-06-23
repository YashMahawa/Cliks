FROM node:20-bookworm-slim AS build

WORKDIR /app
COPY package.json package-lock.json ./
COPY server/package.json server/package.json
COPY site/package.json site/package.json
COPY cli/package.json cli/package.json
RUN npm ci --omit=optional --no-audit --no-fund

COPY . .
RUN npm --workspace @cliks/server run build

FROM node:20-bookworm-slim AS runtime

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=8787

COPY package.json package-lock.json ./
COPY server/package.json server/package.json
COPY site/package.json site/package.json
COPY cli/package.json cli/package.json
RUN npm ci --omit=dev --omit=optional --no-audit --no-fund

COPY --from=build /app/server/dist server/dist

EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:8787/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"

CMD ["node", "server/dist/index.js"]
