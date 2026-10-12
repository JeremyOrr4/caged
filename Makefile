.PHONY: build run test fmt vet install docker-build docker-shell

IMAGE_NAME := caged-dev
VERSION ?= dev

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION)" -o bin/caged ./cmd/caged

run:
	go run ./cmd/caged

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/caged

docker-build:
	docker build -t $(IMAGE_NAME) .

docker-shell:
	docker run --rm -it -v "$(CURDIR):/workspace" -w /workspace $(IMAGE_NAME)
