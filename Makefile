.PHONY: build test check preview-tui preview-app verify package package-all test-package
PROFILE ?= dev/presets/default.json
build:
	go build -o bin/better-scaffold ./cmd/better-scaffold
test:
	go test ./...
	node --test scripts/package.test.mjs scripts/notices.test.mjs
check:
	go vet ./...
preview-tui:
	go run ./cmd/better-scaffold --dry-run --profile "$(PROFILE)"
preview-app:
	go run ./cmd/preview --profile "$(PROFILE)"
verify:
	go run ./cmd/verify
package:
	node scripts/package.mjs
package-all:
	node scripts/package.mjs --all
test-package:
	node --test scripts/package.test.mjs scripts/notices.test.mjs
