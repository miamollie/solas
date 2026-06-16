# syntax=docker/dockerfile:1

FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/greenopsd ./cmd/greenopsd

FROM busybox:1.36.1-musl AS busybox

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/greenopsd /greenopsd
COPY --from=busybox /bin/busybox /busybox

EXPOSE 8000
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD ["/busybox", "sh", "-c", "/busybox wget -qO- http://127.0.0.1:8000/health >/dev/null && /busybox wget -qO- http://127.0.0.1:8000/ready >/dev/null"]
USER nonroot:nonroot
ENTRYPOINT ["/greenopsd"]
