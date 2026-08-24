FROM golang:1.26.2-bookworm

ENV GOPROXY=off GOSUMDB=off CGO_ENABLED=0
WORKDIR /workspace
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
RUN go build -mod=vendor -trimpath -o /usr/local/bin/lockflow ./cmd/lockflow
EXPOSE 19696
CMD ["lockflow", "-address", "0.0.0.0:19696", "-data", "/var/lib/lockflow", "-web", "/workspace/web"]
