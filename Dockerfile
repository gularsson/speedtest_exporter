FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine3.23 AS build

WORKDIR /usr/src

ADD go.mod go.sum ./
RUN go mod download && go mod verify

ADD . ./

RUN go build -o /build/speedtest_exporter ./cmd/speedtest_exporter
FROM alpine:3.23

COPY --from=build /build/speedtest_exporter /usr/local/bin/speedtest_exporter
RUN apk upgrade --no-cache \
    && apk add tzdata

EXPOSE 9090

ENTRYPOINT ["speedtest_exporter"]
