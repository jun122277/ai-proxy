FROM golang:1.27.1-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195 AS development
WORKDIR /src
COPY go.mod Makefile ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY config/ config/
COPY tests/ tests/

FROM development AS build
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/gateway ./cmd/gateway
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/mockllm ./cmd/mockllm

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS runtime-base
LABEL org.opencontainers.image.source="https://github.com/jun122277/ai-proxy"
USER 65532:65532
STOPSIGNAL SIGTERM

FROM runtime-base AS mockllm
COPY --from=build /out/mockllm /mockllm
EXPOSE 9090
ENTRYPOINT ["/mockllm"]
CMD ["-listen", "0.0.0.0:9090"]

FROM runtime-base AS runtime
COPY --from=build /out/gateway /gateway
COPY config/gateway.container.json /etc/ai-proxy/gateway.json
EXPOSE 8080
ENTRYPOINT ["/gateway"]
CMD ["-config", "/etc/ai-proxy/gateway.json"]
