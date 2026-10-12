.PHONY: build run test fmt vet docker-build docker-shell

IMAGE_NAME := caged-dev

build:
	go build -o bin/caged ./cmd/caged

run:
	go run ./cmd/caged

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

docker-build:
	docker build -t $(IMAGE_NAME) .

docker-shell:
	docker run --rm -it -v "$(CURDIR):/workspace" -w /workspace $(IMAGE_NAME)
