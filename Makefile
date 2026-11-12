.PHONY: generate build fmt fmt-check lint tidy-check test cover clean-cache web-check check

# 生成物(api/openapi.yaml)を書く。
generate:
	go generate ./...

# bin/ にバイナリを作る。
build:
	go build -o bin/hedgehug ./cmd/hedgehug

# 書式を直す(gofumpt と goimports)。
fmt:
	golangci-lint fmt ./...

# 書式が崩れていたら落とす(CI と同じ)。
fmt-check:
	golangci-lint fmt --diff ./...

# lint(CI と同じ)。
lint:
	golangci-lint run ./...

# go.mod・go.sum が整っていなければ落とす(CI と同じ)。
tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

# race 付きでテストする。
test:
	gotestsum --format testdox -- -race ./...

# race とカバレッジ付きでテストする(CI がカバレッジとして出すもの)。
cover:
	gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./...

# Go のビルドとテストのキャッシュを空にする。
clean-cache:
	go clean -cache -testcache

# CI が確かめることを手元でまとめて行う。
check: tidy-check lint fmt-check test web-check

# 画面を確かめる: Biome、型、単体テスト、依存の向き、大きさの上限(CI の web と同じ)。
web-check:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm gen
	git diff --exit-code -- web/packages/api/src/generated.ts
	cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size
