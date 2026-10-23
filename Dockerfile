# ---- build ----
# Runs natively on the build machine and cross-compiles for the target platform
# (much faster than building arm64 under QEMU).
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
ARG VERSION=""
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web ./web
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w ${VERSION:+-X main.appVersion=$VERSION}" -o /craftdock .

# ---- runtime ----
# distroless/static includes CA certificates and /tmp (used for mod uploads).
# Runs as root so it can use the mounted Docker socket.
FROM gcr.io/distroless/static-debian12
COPY --from=build /craftdock /craftdock
ENV CRAFTDOCK_DATA=/data
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/craftdock", "-healthcheck"]
ENTRYPOINT ["/craftdock"]
