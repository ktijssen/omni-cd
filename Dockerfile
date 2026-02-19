FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /omni-cd ./cmd/omni-cd

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache \
    git \
    ca-certificates \
    curl \
    jq \
  && OMNICTL_LATEST=$(curl -s https://api.github.com/repos/siderolabs/omni/releases/latest | jq -r .tag_name) \
  && wget -qO /usr/local/bin/omnictl \
    "https://github.com/siderolabs/omni/releases/download/${OMNICTL_LATEST}/omnictl-linux-amd64" \
  && chmod +x /usr/local/bin/omnictl

COPY --from=builder /omni-cd /usr/local/bin/omni-cd

ENTRYPOINT ["omni-cd"]