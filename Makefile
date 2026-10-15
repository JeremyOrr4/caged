.PHONY: build run test fmt vet install clean docker-build docker-shell \
	check fmt-check tidy-check test-race lint vuln cross-build \
	test-release-scripts release-check release-snapshot

IMAGE_NAME := caged-dev
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

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

clean:
	rm -rf bin dist

docker-build:
	docker build -f dev/Dockerfile -t $(IMAGE_NAME) .

docker-shell:
	docker run --rm -it -v "$(CURDIR):/workspace" -w /workspace $(IMAGE_NAME)

# check runs the same steps as CI, in the same order.
check: fmt-check tidy-check vet test-race lint vuln cross-build

fmt-check:
	test -z "$$(gofmt -l .)"

tidy-check:
	go mod tidy -diff

test-race:
	go test -race ./...

lint:
	golangci-lint run

vuln:
	govulncheck ./...

cross-build:
	@for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do \
		echo "build $$target"; \
		GOOS=$${target%/*} GOARCH=$${target#*/} CGO_ENABLED=0 go build -o /dev/null ./cmd/caged || exit 1; \
	done

test-release-scripts:
	bash scripts/next-version_test.sh
	bash scripts/check-pr-commits_test.sh

# release-check runs the project's quality checks and validates both the
# release script and GoReleaser packaging configuration without publishing.
release-check:
	$(MAKE) check
	$(MAKE) test-release-scripts
	@version="$$(goreleaser --version)"; echo "$$version"; \
	  case "$$version" in *2.18.3*) ;; *) echo "expected GoReleaser v2.18.3" >&2; exit 1 ;; esac
	goreleaser check
	$(MAKE) release-snapshot

# release-snapshot builds the release archives locally, without publishing
# anything. Install goreleaser first:
#   go install github.com/goreleaser/goreleaser/v2@v2.18.3
release-snapshot:
	goreleaser release --snapshot --clean
