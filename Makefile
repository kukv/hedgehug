.PHONY: generate build web licenses web-check e2e-bin e2e fmt fmt-check lint tidy-check test cover clean-cache bench-history check release-check

# Write the generated files (api/openapi.yaml).
generate:
	go generate ./...

# Build the GUI into web/dist, which the binary embeds.
web:
	cd web && pnpm install --frozen-lockfile && pnpm build

# Write THIRD_PARTY_LICENSES, which the release archives ship: the license texts
# of the Go standard library and modules, then those of the web bundle (which
# make web writes to web/dist).
licenses: web
	go run ./tools/licenses web/dist/THIRD_PARTY_LICENSES > THIRD_PARTY_LICENSES

# Build the binary into bin/, with the GUI.
build: web
	go build -o bin/hedgehug ./cmd/hedgehug

# Check the GUI: the API types are up to date, biome, svelte-check and the unit tests
# (mirrors the CI web job).
web-check:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm gen
	git diff --exit-code -- web/core/api/schema.d.ts
	cd web && pnpm lint && pnpm check && pnpm test

# Build the binaries the browser tests run: hedgehug with the e2e tag (its keyring
# is a file in HEDGEHUG_HOME, not the OS keychain) and the fake of GitHub. hedgehug
# embeds web/dist, so web is a prerequisite: with make -j, a binary built beside
# make web would embed the bundle of the build before.
e2e-bin: web
	go build -tags e2e -o bin/hedgehug-e2e ./cmd/hedgehug
	go build -o bin/fakegh ./cmd/fakegh

# Run the browser tests against bin/hedgehug-e2e and bin/fakegh.
e2e: e2e-bin
	cd web && pnpm exec playwright install chromium
	cd web && pnpm e2e

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

# Run tests with the race detector and coverage (what CI reports as coverage).
cover:
	gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./...

# Empty the Go build and test caches. Go keeps entries for 5 days and has no
# size limit, so run this when the cache takes too much space.
clean-cache:
	go clean -cache -testcache

# Measure the history index and pack consolidation on a synthesized store of
# 100,000 commits (not part of check or CI; see the history-growth spec).
# The store is made once in $HH_BENCH_CACHE (default: hh-bench in the user cache directory).
bench-history:
	HH_BENCH=1 go test ./internal/app/storage/history -run '^$$' -bench . -benchtime 5x -timeout 60m

# Run everything the CI checks, locally.
check: tidy-check lint fmt-check test web-check

# Validate the release configuration and cross-compilation (mirrors release CI).
release-check:
	goreleaser check
	GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/hedgehug
	GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/hedgehug
