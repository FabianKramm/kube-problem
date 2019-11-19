FROM golang:1.13-alpine as builder

ENV GO111MODULE on
ENV GOFLAGS -mod=vendor

RUN apk add --update --no-cache gcc musl-dev openssl bash gawk sed grep bc coreutils curl

ADD ./vendor /app/vendor
ADD . /app

WORKDIR /app
RUN cd /app && go build -o main main.go && chmod +x main

FROM alpine:latest

RUN mkdir -p /app 
WORKDIR /app

COPY --from=builder /app/main /app

ENTRYPOINT ["/app/main"]
