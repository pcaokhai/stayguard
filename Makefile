# StayGuard task runner. Targets not yet built print the story that adds them and fail.
COMPOSE := docker compose -f deploy/compose.yaml

.PHONY: gen-api gen-sqlc gen-web up down test test-api test-api-int test-web lint lint-api lint-web fmt fmt-api fmt-web licenses gen contracts migrate e2e

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

OAPI_CODEGEN_VERSION := v2.8.0
SQLC_VERSION := v1.31.1

gen: gen-api gen-sqlc gen-web

# Go strict server stubs; the contract is the only input, output is committed (see api/CLAUDE.md).
gen-web:
	cd web && npm run gen

gen-api:
	cd api && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config oapi-codegen.yaml ../contracts/openapi.yaml

# sqlc reads the goose migrations as schema and internal/adapter/postgres/queries; output is committed.
gen-sqlc:
	cd api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate
contracts:
	@bash scripts/contracts.sh
# Runs the binary's own migrate subcommand (embedded goose, forward-only) against MIGRATE_DATABASE_URL, else DATABASE_URL.
# Local dev only (owner superuser, trust auth, RLS bypassed): DATABASE_URL=postgres://stayguard@localhost:5432/stayguard?sslmode=disable make migrate
migrate:
	@test -n "$$DATABASE_URL" || { echo "DATABASE_URL is required: the owner database URL, e.g. postgres://stayguard@localhost:5432/stayguard?sslmode=disable for make up" >&2; exit 1; }
	cd api && go run ./cmd/stayguard migrate

# Testcontainers needs a running Docker daemon. RACE=1 adds -race (needs cgo; CI sets it).
test-api-int:
	cd api && go test $(if $(RACE),-race) -tags integration -count=1 ./...
e2e:
	$(call not_yet,SG-603)
