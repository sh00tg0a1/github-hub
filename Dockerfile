# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS builder
WORKDIR /src
COPY go.mod ./
# No external deps for now; keep step for future modules
RUN --mount=type=cache,target=/go/pkg/mod go mod download || true
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ghh-server ./cmd/ghh-server && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ghh ./cmd/ghh

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata git \
 && addgroup -S app && adduser -S -G app app
COPY --from=builder /out/ghh-server /usr/local/bin/ghh-server
COPY --from=builder /out/ghh /usr/local/bin/ghh
WORKDIR /app
RUN mkdir -p /data && chown -R app:app /data
USER app
EXPOSE 8080
VOLUME ["/data"]
ENV GITHUB_TOKEN=""
ENTRYPOINT ["/usr/local/bin/ghh-server"]
CMD ["--addr", ":8080", "--root", "/data"]