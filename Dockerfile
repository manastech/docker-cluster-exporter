FROM golang:1.12-alpine AS mods
RUN apk add --no-cache git
ADD go.mod /usr/src/docker-cluster-exporter/go.mod
ADD go.sum /usr/src/docker-cluster-exporter/go.sum
WORKDIR /usr/src/docker-cluster-exporter
RUN go mod download

FROM golang:1.12-alpine AS build
ADD . /usr/src/docker-cluster-exporter/
COPY --from=mods /go/pkg/mod/ /go/pkg/mod/
WORKDIR /usr/src/docker-cluster-exporter

RUN apk add --no-cache gcc musl-dev # required to build on arm64
RUN go install docker-cluster-exporter


FROM alpine
COPY --from=build /go/bin/docker-cluster-exporter /usr/bin/docker-cluster-exporter

CMD ["/usr/bin/docker-cluster-exporter"]
