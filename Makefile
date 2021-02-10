PREFLIGHT_BUILD_DIR ?= build
LOG_COLLECTOR_BUILD_DIR ?= dist

clean:
	go clean
	rm -rf $(LOG_COLLECTOR_BUILD_DIR) $(PREFLIGHT_BUILD_DIR)

fmt:
	go fmt ./...

vet:
	go vet ./...

yaml-lint:
	yamllint -c .yamllint ./

shell-lint:
	./hack/run-shell-lint.sh

go-lint:
	golangci-lint run --skip-dirs='(vendor)' -vc ./.golangci.yaml ./...

go-lint-fix:
	golangci-lint run --fix

lint: yaml-lint shell-lint go-lint

go-test:
	GOFLAGS=-mod=vendor ginkgo -v -r -keepGoing ./tests/ -coverprofile coverage.out

test: build go-test

build-log-collector:
	goreleaser release --snapshot --skip-publish --rm-dist

run-log-collector:
	go run ./cmd/main.go

install-required-utilities:
	./hack/install-required-utilities.sh

install: install-required-utilities
	sudo apt-get install curl yamllint
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.30.0
	curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh

build-preflight:
	./hack/build-preflight-artifacts.sh

build: build-preflight build-log-collector

test-preflight-plugin-locally:
	./hack/generate-test-preflight-plugin-manifest.sh
	./hack/test-preflight-plugin-locally.sh

test-log-collector-plugin-locally:
	./hack/generate-test-log-collector-plugin-manifest.sh
	./hack/test-log-collector-plugin-locally.sh

test-preflight: clean build-preflight test-preflight-plugin-locally

test-log-collector: clean build-log-collector test-log-collector-plugin-locally

test-plugins-locally: test-preflight-plugin-locally test-log-collector-plugin-locally

test-plugins-packages: test-preflight test-log-collector

validate-plugin-manifests:
	./hack/validate-plugin-manifests.sh

verify-code-patterns:
	./hack/verify-code-patterns.sh

update-preflight-manifest:
	./hack/update-preflight-manifest.sh

update-log-collector-manifest:
	./hack/update-log-collector-manifest.sh

update-plugin-manifests: update-preflight-manifest update-log-collector-manifest


.PHONY: clean fmt vet go-lint shell-lint go-lint-fix yaml-lint go-test test coverage build run-log-collector
