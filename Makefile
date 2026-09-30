# StayGuard task runner. Targets not yet built print the story that adds them and fail.
COMPOSE := docker compose -f deploy/compose.yaml

.PHONY: up down test test-api test-api-int test-web lint lint-api lint-web fmt fmt-api fmt-web licenses gen contracts migrate e2e

up:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down

test: test-api test-web

test-api:
	cd api && go test ./...

# A missing web/ toolchain must fail loudly, not pass silently (batch B adds web/package.json).
define need_web
	@test -f web/package.json || { echo "web/package.json missing: story SG-001 task 5 adds it" >&2; exit 1; }
endef

test-web:
	$(need_web)
	cd web && npm test

lint: lint-api lint-web

lint-api:
	cd api && golangci-lint run

lint-web:
	$(need_web)
	cd web && npm run lint

fmt: fmt-api fmt-web

fmt-api:
	cd api && gofmt -w .

fmt-web:
	$(need_web)
	cd web && npm run fmt

# go-licenses v2.0.1 is run pinned via `go run`; the npm check is scripts/check-npm-licenses.js (no extra tool).
ALLOWED_LICENSES := Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0

licenses:
	$(need_web)
	cd api && go run github.com/google/go-licenses/v2@v2.0.1 check ./... --ignore github.com/pcaokhai/stayguard --allowed_licenses=$(ALLOWED_LICENSES)
	cd web && node ../scripts/check-npm-licenses.js

define not_yet
	@echo "not yet: story $(1) adds '$@'" >&2; exit 1
endef

gen:
	$(call not_yet,SG-002)
contracts:
	$(call not_yet,SG-002)
migrate:
	$(call not_yet,SG-003)
test-api-int:
	$(call not_yet,SG-003)
e2e:
	$(call not_yet,SG-603)
