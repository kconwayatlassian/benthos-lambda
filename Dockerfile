FROM golang:1.12 AS deps

WORKDIR /go/src/github.com/benthosdev/benthos-lambda/
COPY go.mod /go/src/github.com/benthosdev/benthos-lambda/

ENV GO111MODULE on
RUN go mod download

FROM golang:1.12 AS build

RUN useradd -u 10001 benthos

COPY --from=deps /go/pkg /go/pkg

WORKDIR /go/src/github.com/benthosdev/benthos-lambda/
COPY . /go/src/github.com/benthosdev/benthos-lambda/

ENV GO111MODULE on
RUN CGO_ENABLED=0 GOOS=linux go build -a -o ./cmd/lambda/benthos ./cmd/lambda/main.go

FROM scratch AS package

LABEL maintainer="Ashley Jeffs <ash@jeffail.uk>"

WORKDIR /root/

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/github.com/benthosdev/benthos-lambda/cmd/lambda/benthos .

USER benthos


ENTRYPOINT ["./benthos"]
