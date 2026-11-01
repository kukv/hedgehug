.PHONY: generate build fmt fmt-check lint tidy-check test cover clean-cache check release-check

# Write the generated files (api/openapi.yaml).
generate:
	go generate ./...

# Build the binary into bin/.
build:
	go build -o bin/hedgehug ./cmd/hedgehug

# Auto-format the code (gofumpt + goimports via golangci-lint).
fmt:
	golangci-lint fmt ./...

# Fail if the code is not formatted (mirrors the CI format gate).
fmt-check:
	golangci-lint fmt --diff ./...

# Run the linters (mirrors the CI lint step).
lint:
	golangci-lint run ./...

# Fail if go.mod/go.sum are not tidy (mirrors the CI module-hygiene gate).
tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

# Run tests with the race detector. Coverage is make cover: a build with it is
# another variant in the build cache.
test:
	gotestsum --format testdox -- -race ./...

# Run tests with the race detector and coverage (mirrors the CI test step).
cover:
	gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./...

# Empty the Go build and test caches. Go keeps entries for 5 days and has no
# size limit, so run this when the cache takes too much space.
clean-cache:
	go clean -cache -testcache

# Run everything the CI checks, locally.
check: tidy-check lint fmt-check test

# Validate the release configuration and cross-compilation (mirrors release CI).
release-check:
	goreleaser check
	GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/hedgehug
	GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/hedgehug
