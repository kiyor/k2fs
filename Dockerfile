FROM golang:1.17 as builder
WORKDIR /go/src/k2fs
COPY * ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o k2fs .

FROM alpine
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /root
COPY --from=builder /go/src/k2fs/k2fs .
EXPOSE 8080
ENTRYPOINT ["./k2fs"]
