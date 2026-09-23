FROM golang:1.27.1-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195 AS development
WORKDIR /src
COPY go.mod Makefile ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY config/ config/

FROM development AS build
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS runtime
LABEL org.opencontainers.image.source="https://github.com/jun122277/ai-proxy"
COPY --from=build /out/gateway /gateway
COPY config/gateway.container.json /etc/ai-proxy/gateway.json
USER 65532:65532
EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/gateway"]
CMD ["-config", "/etc/ai-proxy/gateway.json"]
