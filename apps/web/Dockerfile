FROM node:26.5.0-alpine3.23@sha256:0473b6671ff22c8eeb570c0e1e51408595d3171e73f8002c269b763f0a943149 AS build

ARG VITE_API_BASE_URL=/api/v1
ARG VITE_LOCAL_DEMO_MODE=false
ARG VITE_LOCAL_DEMO_IDENTITY
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_LOCAL_DEMO_MODE=${VITE_LOCAL_DEMO_MODE}
ENV VITE_LOCAL_DEMO_IDENTITY=${VITE_LOCAL_DEMO_IDENTITY}

WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web/package.json apps/web/package.json
RUN npm ci
COPY apps/web/ apps/web/
RUN npm --workspace @cumuru/web run build

FROM nginxinc/nginx-unprivileged:1.29.8-alpine3.23@sha256:0c79d56aee561a1d81c63f00eee5fb5fe29279560cdc55e91425133104c7fbe6

ARG BUILD_VERSION
ARG BUILD_REVISION
ARG BUILD_TIME
ARG VITE_LOCAL_DEMO_MODE=false
ARG LOCAL_DEMO_PROXY_LOOPBACK=false

LABEL org.opencontainers.image.version="${BUILD_VERSION}" \
      org.opencontainers.image.revision="${BUILD_REVISION}" \
      org.opencontainers.image.created="${BUILD_TIME}"

USER root
RUN echo "${BUILD_VERSION}" | grep -Eq \
      '^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$' \
  && test "${BUILD_VERSION}" != "unknown" \
  && echo "${BUILD_REVISION}" | grep -Eq \
      '^[0-9A-Za-z][0-9A-Za-z._-]{0,127}$' \
  && test "${BUILD_REVISION}" != "unknown" \
  && echo "${BUILD_TIME}" | grep -Eq \
      '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' \
  && apk add --no-cache \
      c-ares=1.34.8-r0 \
      curl=8.20.0-r0 \
      libcrypto3=3.5.7-r0 \
      libcurl=8.20.0-r0 \
      libexpat=2.8.2-r0 \
      libpng=1.6.58-r1 \
      libssl3=3.5.7-r0 \
      libxml2=2.13.9-r1

COPY deploy/nginx/default.conf /tmp/cumuru-default.conf.template
RUN case "${VITE_LOCAL_DEMO_MODE}:${LOCAL_DEMO_PROXY_LOOPBACK}" in \
      false:false) proxy_client_address='$remote_addr' ;; \
      true:true) proxy_client_address='127.0.0.1' ;; \
      *) echo "local demo proxy and web build modes must match" >&2; exit 1 ;; \
    esac \
  && export CUMURU_PROXY_CLIENT_ADDRESS="${proxy_client_address}" \
  && envsubst '${CUMURU_PROXY_CLIENT_ADDRESS}' \
      < /tmp/cumuru-default.conf.template \
      > /etc/nginx/conf.d/default.conf \
  && rm /tmp/cumuru-default.conf.template
COPY --from=build --chown=101:101 /src/apps/web/dist /usr/share/nginx/html

USER 101:101
EXPOSE 8080
