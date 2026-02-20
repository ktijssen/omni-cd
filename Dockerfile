FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG APP_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${APP_VERSION}" -o /omni-cd ./cmd/omni-cd

# Runtime stage
FROM alpine:3.20

ARG OMNICTL_VERSION

RUN apk add --no-cache \
    git \
    ca-certificates \
    curl \
    jq \
  && OMNICTL_VER="${OMNICTL_VERSION:-$(curl -s https://api.github.com/repos/siderolabs/omni/releases/latest | jq -r .tag_name)}" \
  && wget -qO /usr/local/bin/omnictl \
    "https://github.com/siderolabs/omni/releases/download/${OMNICTL_VER}/omnictl-linux-amd64" \
  && chmod +x /usr/local/bin/omnictl

COPY --from=builder /omni-cd /usr/local/bin/omni-cd

ENTRYPOINT ["omni-cd"]