# Build the manager binary
FROM golang:1.22.7-alpine as base
# Copy in the go src
WORKDIR /go/src/github.com/trilioData/tvk-plugins

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

COPY cmd/preflight cmd/preflight
COPY tools/preflight tools/preflight
COPY internal/ internal/
COPY vendor/ vendor/

RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=vendor go build -a -o preflight github.com/trilioData/tvk-plugins/cmd/preflight

FROM registry.access.redhat.com/ubi8/ubi:latest as final
LABEL name=tvk-plugins-preflight
LABEL summary=TVK\ Plugins\ Preflight
LABEL description=TVK\ Plugins\ Preflight

# RUN addgroup -S usergroup && adduser -DH -g "" -u 1001 -S docker -G usergroup
# USER docker
WORKDIR /
RUN mkdir -p /opt/tvk-plugins
COPY --from=base /go/src/github.com/trilioData/tvk-plugins/preflight /opt/tvk-plugins/preflight
