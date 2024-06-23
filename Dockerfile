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
RUN wget https://github.com/CESSProject/DeOSS/releases/download/v0.3.6/DeOSS0.3.6.linux-amd64.tar.gz && tar -xvf DeOSS0.3.6.linux-amd64.tar.gz && rm -rf DeOSS0.3.6.linux-amd64.tar.gz


# create running image
FROM debian:bullseye-slim As latest

# copy execuable mgtbe file from builder image to current dir
COPY --from=builder /work/DeOSS0.3.6.linux-amd64 /deoss
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

