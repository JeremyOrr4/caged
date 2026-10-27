.PHONY: build run test fmt vet clean docker-build docker-shell \
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

# End-to-end tests against real Docker (dev/e2e/README.md). The scratch folder
# must be outside the repository and, on macOS, shared with Docker Desktop
# (/Users is by default). The image guard saves the daemon's caged images
# first and loads them back at the end; E2E_SAVE_IMAGES=0 makes it refuse
# instead.
.PHONY: e2e-selftest e2e-smoke e2e e2e-macos try
E2E_SCRATCH ?= $(HOME)/.cache/caged-e2e
E2E_SAVE_IMAGES ?= 1
E2E = E2E_SAVE_IMAGES=$(E2E_SAVE_IMAGES) bash dev/e2e/e2e.sh "$(E2E_SCRATCH)" "$(CURDIR)"

e2e-selftest:
	bash dev/e2e/selftest.sh

# e2e-smoke runs section S only: build, run, doctor, clean (about 2 minutes).
e2e-smoke: e2e-selftest
	$(E2E) S

e2e: e2e-selftest
	$(E2E)

# e2e-macos is e2e on a Mac, where section M (macOS only) runs too.
e2e-macos: e2e-selftest
	@test "$$(uname -s)" = Darwin || { echo "e2e-macos needs macOS; run make e2e here" >&2; exit 1; }
	@case "$(E2E_SCRATCH)" in /Users/*) ;; *) echo "E2E_SCRATCH must be under /Users (Docker Desktop file sharing)" >&2; exit 1;; esac
	$(E2E)

# try builds bin/caged and runs it with ARGS, e.g. make try ARGS=doctor.
try:
	scripts/run-local.sh $(ARGS)
