# 构建阶段（注意：go.mod go 指令 1.25.0，基础镜像必须 ≥ 1.25，与本地工具链对齐）
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
# 可覆盖：CI 默认官方 proxy；国内构建可 --build-arg GOPROXY=https://goproxy.cn,direct
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY=${GOPROXY} go mod download
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
