GO_SRCS := $(shell find . -type f -name '*.go' -a -name '*.tpl' -a ! \( -name 'zz_generated*' -o -name '*_test.go' \))
GO_TESTS := $(shell find . -type f -name '*_test.go')
TAG_NAME = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
TAG_NAME_DEV = $(shell git describe --tags --abbrev=0 2>/dev/null)
VERSION_CORE = $(shell echo $(TAG_NAME) | sed 's/^\(v[0-9]\+\.[0-9]\+\.[0-9]\+\)\(+\([0-9]\+\)\)\?$$/\1/')
VERSION_CORE_DEV = $(shell echo $(TAG_NAME_DEV) | sed 's/^\(v[0-9]\+\.[0-9]\+\.[0-9]\+\)\(+\([0-9]\+\)\)\?$$/\1/')
GIT_COMMIT = $(shell git rev-parse --short=7 HEAD)
VERSION = $(or $(and $(TAG_NAME),$(VERSION_CORE)),$(and $(TAG_NAME_DEV),$(VERSION_CORE_DEV)-dev),$(GIT_COMMIT))
VERSION_NO_V = $(shell echo $(VERSION) | sed 's/^v\(.*\)$$/\1/')

golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := $(shell go env GOPATH)/bin/golangci-lint
endif

pkgsite := $(shell which pkgsite)
ifeq ($(pkgsite),)
pkgsite := $(shell go env GOPATH)/bin/pkgsite
endif


.PHONY: bin/withny-dl
bin/withny-dl: $(GO_SRCS)
	CGO_ENABLED=1 go build -trimpath -ldflags '-X main.version=${VERSION} -s -w' -o "$@" ./main.go

.PHONY: bin/withny-dl-static
bin/withny-dl-static: $(GO_SRCS)
	CGO_ENABLED=1 go build -trimpath -ldflags '-X main.version=${VERSION} -s -w -extldflags "-lswresample -static"' -o "$@" ./main.go

.PHONY: bin/withny-dl-static.exe
bin/withny-dl-static.exe: $(GO_SRCS)
	CGO_ENABLED=1 \
	GOOS=windows \
	GOARCH=amd64 \
	go build -trimpath -ldflags '-X main.version=${VERSION} -linkmode external -s -w -extldflags "-static"' -o "$@" ./main.go

.PHONY: bin/withny-dl-darwin
bin/withny-dl-darwin: $(GO_SRCS)
	CGO_ENABLED=1 \
	GOOS=darwin \
	go build -trimpath -ldflags '-X main.version=${VERSION} -linkmode external -s -w' -o "$@" ./main.go

.PHONY: all
all: $(addprefix bin/,$(bins))

.PHONY: unit
unit:
	go test -race -covermode=atomic -tags=unit -timeout=30s ./...

.PHONY: coverage
coverage: $(GO_TESTS)
	go test -race -covermode=atomic -tags=unit -timeout=30s -coverprofile=coverage.out ./...
	go tool cover -html coverage.out -o coverage.html

.PHONY: integration
integration:
	go test -race -covermode=atomic -tags=integration -timeout=300s ./...

$(golint):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

$(pkgsite):
	go install golang.org/x/pkgsite/cmd/pkgsite@latest

.PHONY: lint
lint: $(golint)
	$(golint) run ./...

.PHONY: clean
clean:
	rm -rf bin/
	rm -rf target/

.PHONY: package
package: target \
	target/checksums.txt \
	target/release.md

.PHONY: target
target: target-darwin \
	target-static \
	target-static-windows

target/checksums.txt: target
	sha256sum -b $(addsuffix /withny*,$^) | sed 's|*target/||' > $@

target/release.md: target/checksums.txt
	sed -e '/@@@CHECKSUMS@@@/{r target/checksums.txt' -e 'd}' .github/RELEASE_TEMPLATE.md > $@

target/withny-dl-linux-amd64 target/withny-dl-linux-arm64 target/withny-dl-linux-riscv64:
	podman manifest rm localhost/builder:static || true
	mkdir -p ./target
	podman build \
		--manifest localhost/builder:static \
		--jobs=2 --platform=linux/amd64,linux/arm64/v8,linux/riscv64 \
		--target export \
		--output=type=local,dest=./target \
		-f Dockerfile.static .
	./assert-arch.sh

.PHONY: target-static
target-static: target/withny-dl-linux-amd64 target/withny-dl-linux-arm64 target/withny-dl-linux-riscv64

target/withny-dl-windows-amd64.exe:
	mkdir -p ./target
	podman build \
		-t localhost/builder:static-windows \
		--target export \
		--output=type=local,dest=./target \
		-f Dockerfile.static-windows .
	./assert-arch.sh

.PHONY: target-static-windows
target-static-windows: target/withny-dl-windows-amd64.exe

target/withny-dl-darwin-amd64 target/withny-dl-darwin-arm64:
	podman manifest rm localhost/builder:darwin || true
	mkdir -p ./target
	podman build \
		--manifest localhost/builder:darwin \
		--jobs=2 --platform=linux/amd64,linux/arm64/v8 \
		--target export \
		--output=type=local,dest=./target \
		-f Dockerfile.darwin .
	./assert-arch.sh

.PHONY: target-darwin
target-darwin: target/withny-dl-darwin-amd64 target/withny-dl-darwin-arm64

.PHONY: docker-static
docker-static:
	podman manifest rm ghcr.io/darkness4/withny-dl:latest || true
	podman build \
		--manifest ghcr.io/darkness4/withny-dl:latest \
		--jobs=2 --platform=linux/amd64,linux/arm64/v8,linux/riscv64 \
		-f Dockerfile.static .
	podman manifest push --all ghcr.io/darkness4/withny-dl:latest "docker://ghcr.io/darkness4/withny-dl:latest"
	podman manifest push --all ghcr.io/darkness4/withny-dl:latest "docker://ghcr.io/darkness4/withny-dl:${VERSION_NO_V}"
	podman manifest push --all ghcr.io/darkness4/withny-dl:latest "docker://ghcr.io/darkness4/withny-dl:dev"

.PHONY: version
version:
	echo version=$(VERSION)

.PHONY: memleaks
memleaks:
	cd video/probe && make clean && make valgrind
	cd video/concat && make clean && make valgrind

.PHONY: doc
doc: $(pkgsite)
	$(pkgsite) -open .
