FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# goose from the tool pinned in go.mod; only built when the migrate target is requested.
FROM build AS goose-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=goose-build /out/goose /goose
COPY internal/db/migrations /migrations
ENV GOOSE_DRIVER=postgres \
    GOOSE_MIGRATION_DIR=/migrations
# GOOSE_DBSTRING is provided at runtime.
ENTRYPOINT ["/goose"]
CMD ["up"]

FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
