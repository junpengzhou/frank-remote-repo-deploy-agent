# syntax=docker/dockerfile:1.7

# 使用官方 Go 镜像做编译环境；最终产物设置 CGO_ENABLED=0，生成适合 CentOS 直接运行的 Linux 静态二进制。
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS builder

WORKDIR /src

# 先复制依赖文件，方便 Docker 缓存 go mod download。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
      -o /out/salt-agent \
      ./cmd/salt-agent

RUN chmod +x /out/salt-agent && \
    sha256sum /out/salt-agent > /out/salt-agent.sha256

# export stage 专门配合 docker build --output，把 /out 内文件导出到宿主机 build/centos。
FROM scratch AS export
COPY --from=builder /out/ /
