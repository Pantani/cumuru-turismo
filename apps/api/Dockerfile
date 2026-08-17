FROM golang:1.26.6-alpine3.23@sha256:e57c41c1d5864341031181b0db34b9a537bb5773eb6428e4e5bdaea0f9135406 AS build

ARG BUILD_VERSION
ARG BUILD_REVISION
ARG BUILD_TIME

WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./

RUN echo "${BUILD_VERSION}" | grep -Eq \
       '^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$' \
  && test "${BUILD_VERSION}" != "unknown" \
  && echo "${BUILD_REVISION}" | grep -Eq \
       '^[0-9A-Za-z][0-9A-Za-z._-]{0,127}$' \
  && test "${BUILD_REVISION}" != "unknown" \
  && echo "${BUILD_TIME}" | grep -Eq \
       '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' \
  && CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${BUILD_VERSION} -X main.revision=${BUILD_REVISION} -X main.builtAt=${BUILD_TIME}" \
      -o /out/api ./cmd/api \
  && CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${BUILD_VERSION} -X main.revision=${BUILD_REVISION} -X main.builtAt=${BUILD_TIME}" \
      -o /out/worker ./cmd/worker \
  && CGO_ENABLED=0 go build -trimpath \
      -o /out/local-demo ./cmd/localdemo \
  && CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w" \
      -o /out/seed ./cmd/seed \
  && CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w" \
      -o /out/configcheck ./cmd/configcheck

FROM alpine:3.23.5@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG BUILD_VERSION
ARG BUILD_REVISION
ARG BUILD_TIME

LABEL org.opencontainers.image.version="${BUILD_VERSION}" \
      org.opencontainers.image.revision="${BUILD_REVISION}" \
      org.opencontainers.image.created="${BUILD_TIME}"

RUN apk add --no-cache \
      ca-certificates \
      tzdata=2026c-r0 \
  && addgroup -S -g 10001 cumuru \
  && adduser -S -D -H -u 10001 -G cumuru cumuru

WORKDIR /app
COPY --from=build --chown=cumuru:cumuru /out/api /app/api
COPY --from=build --chown=cumuru:cumuru /out/worker /app/worker
COPY --from=build --chown=cumuru:cumuru /out/local-demo /app/local-demo
COPY --from=build --chown=cumuru:cumuru /out/seed /app/seed
COPY --from=build --chown=cumuru:cumuru /out/configcheck /app/configcheck

USER 10001:10001
CMD ["/app/api"]
