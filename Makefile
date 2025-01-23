BUILD_DIR ?= build
DIST_DIR ?= dist

clean:
	go clean
	rm -rf $(DIST_DIR) $(BUILD_DIR)

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

install-required-utilities:
	./hack/install-required-utilities.sh

install: install-required-utilities
ifeq ($(shell uname), Darwin)
	brew install curl yamllint goreleaser
else
	echo "deb [trusted=yes] https://repo.goreleaser.com/apt/ /" | sudo tee /etc/apt/sources.list.d/goreleaser.list
	sudo apt-get update
	sudo apt-get install curl yamllint goreleaser
endif
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.30.0

build-preflight:
	find . -name .goreleaser.yml -exec sed -i '/binary: log-collector/a \ \ skip: true' {} +
	goreleaser release --snapshot --skip=publish --clean
	find . -name .goreleaser.yml -exec sed -i '/skip: true/d' {} +

build-cleanup:
	./hack/build-cleanup-artifacts.sh

build-log-collector:
	find . -name .goreleaser.yml -exec sed -i '/binary: preflight/a \ \ skip: true' {} +
	goreleaser release --snapshot --skip=publish --clean
	find . -name .goreleaser.yml -exec sed -i '/skip: true/d' {} +

build: build-preflight build-cleanup
	goreleaser release --snapshot --skip publish --clean


test-logcollector-unit:
	./hack/run-unit-tests.sh cmd/log-collector/cmd/...

test-preflight-plugin-locally:
	./hack/generate-test-preflight-plugin-manifest.sh
	./hack/test-preflight-plugin-locally.sh

test-cleanup-plugin-locally:
	./hack/generate-test-cleanup-plugin-manifest.sh
	./hack/test-cleanup-plugin-locally.sh

test-log-collector-plugin-locally:
	./hack/generate-test-log-collector-plugin-manifest.sh
	./hack/test-log-collector-plugin-locally.sh


test-preflight-unit:
	./hack/run-unit-tests.sh cmd/preflight/cmd/...
	./hack/run-unit-tests.sh tools/preflight/...

test-preflight-integration:
	./hack/run-integration-tests.sh tests/preflight/...

test-cleanup-integration:
	./tests/cleanup/cleanup_test.sh

test: test-preflight-integration  test-cleanup-integration

test-preflight: clean build-preflight test-preflight-plugin-locally

test-cleanup: clean build-cleanup test-cleanup-integration test-cleanup-plugin-locally

test-log-collector: clean build-log-collector test-log-collector-plugin-locally

test-plugins-locally: test-preflight-plugin-locally test-log-collector-plugin-locally test-cleanup-plugin-locally

test-plugins-packages: test-preflight test-log-collector test-cleanup

validate-plugin-manifests:
	./hack/validate-plugin-manifests.sh

verify-code-patterns:
	./hack/verify-code-patterns.sh

update-preflight-manifest:
	./hack/update-preflight-manifest.sh

update-cleanup-manifest:
	./hack/update-cleanup-manifest.sh

update-log-collector-manifest:
	./hack/update-log-collector-manifest.sh


update-plugin-manifests: update-preflight-manifest update-log-collector-manifest update-cleanup-manifest

ready: fmt vet lint verify-code-patterns

.PHONY: clean fmt vet go-lint shell-lint go-lint-fix yaml-lint go-test test coverage build run-log-collector
