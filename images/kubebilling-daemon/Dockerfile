FROM golang:1.18.4 AS build

WORKDIR /go/build

COPY go.mod go.sum ./

ENV GO111MODULE on
ENV GOPROXY https://goproxy.cn

RUN go env -w GO111MODULE=on \
    && go mod download

COPY ./ /go/build

RUN go build -o /opt/kubebilling-daemon cmd/kubebilling-daemon/main.go

FROM nouchka/sqlite3:latest

COPY --from=build /opt/kubebilling-daemon /usr/local/bin/kubebilling-daemon