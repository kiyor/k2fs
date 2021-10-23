FROM golang:1.17 as builder
WORKDIR /go/src/k2fs
COPY vendor ./vendor
COPY * ./
RUN CGO_ENABLED=0 GOOS=linux go build -mod vendor -a -installsuffix cgo -o k2fs .

FROM alpine
RUN apk update && apk add ca-certificates su-exec && rm -rf /var/cache/apk/*
WORKDIR /bin
COPY --from=builder /go/src/k2fs/k2fs .
EXPOSE 8080
CMD ["./k2fs"]
