# Frontend build stage
FROM node:26-alpine AS frontend

WORKDIR /frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

# Go build stage
FROM golang:1.26.2-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
COPY --from=frontend /internal/web/dist ./internal/web/dist
ARG APP_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${APP_VERSION}" -o /omni-cd ./cmd/omni-cd

# Runtime stage
FROM alpine:3.23

RUN apk add --no-cache \
    git \
    ca-certificates

COPY --from=builder /omni-cd /usr/local/bin/omni-cd

ENTRYPOINT ["omni-cd"]