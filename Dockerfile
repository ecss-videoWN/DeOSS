# create build image
FROM golang:alpine AS builder

# set golang env
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# cd to work dir
WORKDIR /work

# copy source code file to WORKDIR
COPY . ./

# build
RUN go build -o deoss cmd/main.go

# create running image
FROM debian:bullseye-slim As latest

# copy execuable mgtbe file from builder image to current dir
COPY --from=builder /work/deoss /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

RUN apt install git curl wget vim util-linux -y

