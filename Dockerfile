FROM golang:1.26-alpine AS build
WORKDIR /src

# Dependencies first so this layer stays cached until go.mod changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

# Distroless static: no shell or package manager, runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /api /api
USER nonroot:nonroot
ENTRYPOINT ["/api"]
