# Go 1.27.3, pinned official-library replica; override only for an approved mirror.
ARG GO_IMAGE=public.ecr.aws/docker/library/golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
ENV GOMAXPROCS=2
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY internal ./internal
COPY configs ./configs
COPY beacon.sh compose.yaml compose.beacon.yaml compose.dev.yaml ./
COPY deploy/systemd ./deploy/systemd
COPY deploy/trust/roots ./deploy/trust/roots
RUN CGO_ENABLED=0 go test -p 2 ./... && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -p 2 -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /beacon ./cmd/rf-link-server-beacon

RUN mkdir -p /volumes/data /volumes/exchange /volumes/dashboard && chmod 700 /volumes/data && chmod 755 /volumes/exchange && chmod 2750 /volumes/dashboard && touch /volumes/data/.volume /volumes/exchange/.volume /volumes/dashboard/.volume

FROM scratch AS runtime
COPY --from=build --chown=65532:65532 /volumes/data /data
COPY --from=build --chown=65532:65532 /volumes/exchange /exchange
COPY --from=build --chown=65532:65532 /src/deploy/trust/roots /exchange/roots
COPY --from=build --chown=65532:1000 /volumes/dashboard /dashboard
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /beacon /rf-link-server-beacon
USER 65532:65532
ENTRYPOINT ["/rf-link-server-beacon"]
CMD ["status"]


FROM build AS compose-fixture
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go test -c -o /compose-fixture ./internal/managed

FROM runtime AS integration
COPY --from=compose-fixture /compose-fixture /compose-fixture

FROM runtime AS release
