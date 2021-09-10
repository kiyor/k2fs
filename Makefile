VERSION := $(shell cat ./VERSION)
LDFLAGS := -ldflags "-w -s"

release:
	git tag -a $(VERSION) -m "release" || true
	git push origin master --tags
.PHONY: release

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -v ${LDFLAGS} -o ./k2fs .
.PHONY: build

image:
	docker build -t kiyor/k2fs . && docker push kiyor/k2fs
.PHONY: image
