# Build the manager binary
FROM golang:1.12.5 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY metrics/ metrics/
COPY store/ store/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Use alpine as minimal base image to package the manager binary
FROM alpine:latest
WORKDIR /
COPY --from=builder /workspace/manager .
RUN mkdir -p /root/.kube/
ENTRYPOINT ["/manager"]
