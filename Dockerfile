FROM golang:1.18-alpine3.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY main.go main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags '-extldflags "-static"' -o rpi-image main.go

FROM alpine:3.16

COPY --from=builder /workspace/rpi-image .

WORKDIR /workspace
ENTRYPOINT ["/rpi-image"]
