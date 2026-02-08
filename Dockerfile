FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine3.23 AS build

WORKDIR /usr/src

ADD go.mod go.sum ./
RUN go mod download && go mod verify

ADD . ./

RUN go build -o ./cmd/speedtest_exporter/

FROM alpine:3.23

COPY --from=build /usr/src/cmd/speedtest_exporter /usr/local/bin/speedtest_exporter
RUN apk upgrade --no-cache \
    && apk add tzdata

EXPOSE 9090

ENTRYPOINT ["speedtest_exporter"]
