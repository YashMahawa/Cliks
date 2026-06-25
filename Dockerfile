FROM golang:1.24-bookworm AS build

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server ./
RUN go build -trimpath -ldflags="-s -w" -o /out/cliks-server .

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
ENV PORT=8787

COPY --from=build /out/cliks-server /app/cliks-server

EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8787/health >/dev/null || exit 1

CMD ["/app/cliks-server"]
