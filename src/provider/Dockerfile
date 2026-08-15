# 构建阶段
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server

# 运行阶段：非 root、仅含二进制与证书
FROM alpine:3.20
RUN adduser -D -u 10001 app && apk add --no-cache ca-certificates
USER app
WORKDIR /app
COPY --from=build /out/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
