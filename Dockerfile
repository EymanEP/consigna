# syntax=docker/dockerfile:1

# Build the web UI.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build the binary with the UI embedded.
FROM golang:1.24-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/
COPY --from=web /src/web/dist/ web/dist/
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /consigna ./cmd/consigna

# A minimal, non-root runtime image.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /consigna /consigna
USER nonroot:nonroot
EXPOSE 7431
VOLUME ["/data"]
# On a LAN, run with --network host so Consigna sees (and shows) the real
# network addresses. See docs/self-hosting.md.
ENTRYPOINT ["/consigna", "--data-dir", "/data", "--no-settings-file", "--no-qr"]
