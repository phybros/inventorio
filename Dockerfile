# syntax=docker/dockerfile:1

FROM golang:1.26.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inventorio .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S inventorio \
	&& adduser -S -G inventorio inventorio

USER inventorio
WORKDIR /app

COPY --from=build /out/inventorio /app/inventorio

ENV LISTEN_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/app/inventorio"]
