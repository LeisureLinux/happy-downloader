FROM node:24-slim AS ui
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack prepare pnpm@11.4.0 --activate && corepack enable
COPY . /usr/src/happy-downloader

WORKDIR /usr/src/happy-downloader/frontend

RUN rm -rf node_modules

RUN pnpm install
RUN pnpm build



FROM golang AS build

WORKDIR /usr/src/happy-downloader

COPY . .
COPY --from=ui /usr/src/happy-downloader/frontend /usr/src/happy-downloader/frontend

RUN CGO_ENABLED=0 GOOS=linux go build -o happy-downloader



FROM python:alpine

RUN apk update && \
    apk add ffmpeg ca-certificates curl wget gnutls deno --no-cache && \
    pip install "yt-dlp[default,curl-cffi,mutagen,pycryptodomex,phantomjs,secretstorage]"

VOLUME /downloads /config

WORKDIR /app

COPY --from=build /usr/src/happy-downloader/happy-downloader /app

ENV APP_PATHS_DOWNLOAD_PATH="/downloads"
ENV APP_PATHS_LOCAL_DATABASE_PATH="/config"
ENV APP_PATHS_JS_RUNTIME_PATH="deno:/usr/bin/deno"

EXPOSE 3033
ENTRYPOINT [ "./happy-downloader", "--conf", "/config/config.yml"]
