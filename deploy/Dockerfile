# Build context is the repository root (see deploy/compose.yaml).
FROM node:26.10.0-alpine AS web
# Production builds pass --build-arg NEXT_PUBLIC_DEMO_MODE=0 to hide the demo role picker on sign-in.
ARG NEXT_PUBLIC_DEMO_MODE=1
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# The web type-check reaches the shared contract files by relative path (../../../../../contracts from web/src/...).
COPY contracts/ /contracts/
RUN npm run build

FROM golang:1.27.1-alpine AS api
WORKDIR /src
COPY api/go.mod api/go.sum* ./
RUN go mod download
COPY api/ ./
# Static binary so the runtime image needs no libc.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/stayguard ./cmd/stayguard

# Distroless "nonroot" runs as uid 65532; there is no shell, package manager or root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/stayguard /app/stayguard
COPY --from=web /web/out /app/web
# Must match the layout above; the config default (web/out) is for local runs from the repo root.
ENV STATIC_DIR=/app/web
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app/stayguard"]
