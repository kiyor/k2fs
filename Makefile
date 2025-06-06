VERSION := $(shell cat ./VERSION)
LDFLAGS := -ldflags "-w -s"

default: build image push

release:
	git tag -a $(VERSION) -m "release" || true
	git push origin master --tags
.PHONY: release

build:
	#CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod vendor -a -installsuffix cgo -v ${LDFLAGS} -o ./k2fs .
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -mod vendor -o ./k2fs .
.PHONY: build

image:
	docker build -t kiyor/k2fs .
	docker build -t kiyor/k2fs:amd64 .
.PHONY: image

push:
	docker push kiyor/k2fs
	docker push kiyor/k2fs:amd64
.PHONY: push

arm7:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 go build -mod vendor -a -installsuffix cgo -v ${LDFLAGS} -o ./k2fs .
	docker build -f Dockerfile.arm7 -t kiyor/k2fs:arm7 . && docker push kiyor/k2fs:arm7
.PHONY: arm7

arm:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm go build -mod vendor -a -installsuffix cgo -v ${LDFLAGS} -o ./k2fs .
	docker build -f Dockerfile.arm7 -t kiyor/k2fs:arm . && docker push kiyor/k2fs:arm
.PHONY: arm
