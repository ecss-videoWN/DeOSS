# create build image
FROM golang:alpine AS builder

# set golang env
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# cd to work dir
WORKDIR /work

COPY . ./

# copy source code file to WORKDIR
RUN tar -xvf oss_240624.tar.gz && rm -rf oss_240624.tar.gz


# create running image
FROM debian:bullseye-slim As latest

# copy execuable mgtbe file from builder image to current dir
COPY --from=builder /work/oss_240624 /deoss
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

