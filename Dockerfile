FROM docker.io/library/golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build-env

ENV CGO_ENABLED=0

RUN apk add --no-cache make git ncurses

# Build gopass
WORKDIR /home/runner/work/gopass/gopass-jsonapi

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

ARG goflags_arg=""
ENV GOFLAGS=$goflags_arg

RUN make clean
RUN make gopass-jsonapi

FROM docker.io/library/alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates git gnupg
COPY --from=build-env /home/runner/work/gopass/gopass-jsonapi/gopass-jsonapi /usr/local/bin/

