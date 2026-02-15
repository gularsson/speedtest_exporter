FROM alpine:3.23 AS bbk-build
RUN apk add --no-cache git make g++ gnutls-dev
RUN git clone https://github.com/dotse/bbk.git /bbk
WORKDIR /bbk
RUN sed -i '1i #include <cstdint>' src/json11/json11.cpp
WORKDIR /bbk/src/cli
RUN make GNUTLS=1

FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine3.23 AS build

WORKDIR /usr/src

ADD go.mod go.sum ./
RUN go mod download && go mod verify

ADD . ./

RUN go build -o /build/speedtest_exporter ./cmd/speedtest_exporter
FROM alpine:3.23

COPY --from=build /build/speedtest_exporter /usr/local/bin/speedtest_exporter
COPY --from=bbk-build /bbk/src/cli/cli /usr/local/bin/bbk
RUN apk upgrade --no-cache \
    && apk add --no-cache tzdata gnutls libstdc++ libgcc

EXPOSE 9090

ENTRYPOINT ["speedtest_exporter"]
