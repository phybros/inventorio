# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildDate=$BUILD_DATE" -o /out/inventorio .

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
	&& addgroup -S inventorio \
	&& adduser -S -G inventorio inventorio

USER inventorio
WORKDIR /app

COPY --from=build /out/inventorio /app/inventorio

ENV LISTEN_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/app/inventorio"]
