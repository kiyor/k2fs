FROM golang:1.18 as builder
WORKDIR /go/src/k2fs
COPY vendor ./vendor
COPY lib ./lib
COPY pkg ./pkg
COPY go.mod go.mod
COPY go.sum go.sum
#COPY app.js app.js
COPY *.go ./
COPY *.html ./
COPY *.js ./
COPY *.css ./
RUN go build -mod vendor -a -installsuffix cgo -o k2fs .

#FROM alpine:3.3
#RUN apk update && apk add ca-certificates su-exec unzip unrar tzdata && rm -rf /var/cache/apk/*
FROM ubuntu
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && apt-get install -qqy \
    apt-transport-https \
    ca-certificates \
#    su-exec \
    curl \
    unzip \
    unrar \
    locales
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && \
    locale-gen
ADD https://github.com/tianon/gosu/releases/download/1.14/gosu-amd64 /usr/local/bin/su-exec
RUN chmod +x /usr/local/bin/su-exec

ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
WORKDIR /bin
COPY --from=builder /go/src/k2fs/k2fs .
COPY conv .
COPY local local
EXPOSE 8080
CMD ["./k2fs"]
