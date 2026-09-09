APP_NAME=api

include .env
export

help:
	@echo "Available Commands:\n"

	@echo "Help Commands:"
	@echo "  make --help   - Show make commands"
	@echo "  make --go-help   - Show a named go command\n"

	@echo "Dependency Management:"
	@echo "  make init   - Initialize go module"
	@echo "  make get   - Add dependency to the module"
	@echo "  make update   - Update the the named dependency in the module to its latest version"
	@echo "  make install   - Install a dependency to the system"
	@echo "  make download  - Download required modules and dependencies"
	@echo "  make fix   - Update and fix obsolete syntax used in go files"
	@echo "  make tidy   - Add missing and clean unused dependencies"
	@echo "  make list   - List all packages in the module"
	@echo "  make list-json   - List all packages in the module in json format\n"

	@echo "Cache Management:"
	@echo "  make clean-cache   - Delete cached go data"
	@echo "  make clean-test-cache   - Delete cached go test data\n"

	@echo "API Documentation"
	@echo "  make swagger   - Document API in swagger\n"

	@echo "Code Testing & Fixing:"
	@echo "  make test   - Perform all unit tests recursively"
	@echo "  make test-race   - Perform race condition test on all files recursively"
	@echo "  make test-coverage   - Perform test coverage and write results to coverage.out file"
	@echo "  make vet   - Scan erronous code\n"

	@echo "Code Formatting & Linting:"
	@echo "  make fmt   - Format all go source files"
	@echo "  make gofmt   - Format all go source files in the named directory\n"
	

	@echo "SQLC:"
	@echo "  make sqlc   - Generate SQLC code and queries\n"

	@echo "Database Migrations:"
	@echo "  make migrate-create   - Create SQL migration files"
	@echo "  make migrate-up   - Apply migrations"
	@echo "  make migrate-down   - Drop migrations"
	@echo "  make migrate-force-version   - Force migration version to the named value"
	@echo "  make app-migrate-up   - Apply migrations using migrate binary"
	@echo "  make app-migrate-down   - Drop migrations using migrate binary"
	@echo "  make app-migrate-version-force   - Force migration version to the named value using migrate binary\n"

	@echo "Security & Vulnarability Scan:"
	@echo "  make gosec   - Run security scan on go source file"
	@echo "  make govulncheck  - Run vulnarability check on go source file\n"



	# Database migrations
	@echo "  make migrate-up  - Apply migrations"
	@echo "  make migrate-down  - Downgrade migrations"
	@echo "  make migrate-create  - Create migrations"

--help:
	make --help

--go-help:
	go help $(cmd)

# ===============================
#	Dependency Management
# ===============================

init:
	go mod init $(name)
	
get:
	go get $(name)

update:
	go get -u $(name)

install:
	go install $(name)

download:
	go mod download

fix:
	go fix ./...

tidy:
	go mod tidy

list:
	go list -m all

list-json:
	go list -json ./...

# ===============================
#	Cache Managaement
# ===============================

clean-cache:
	go clean -cache

clean-test-cache:
	go clean -testcache

# ===============================
#	Build & Run
# ===============================

build-api:
	mkdir -p bin
	go build -o bin/api ./cmd/api


build-migrate:
	mkdir -p bin
	go build -o bin/migrate ./cmd/migrate

run-api:
	go run ./cmd/api/main.go

run-migrate:
	go run .cmd/migrate/main.go

# ===============================
#	API Documentation
# ===============================

swagger:
	swagger generate spec -o ./docs/swagger.yaml --scan-models

# ===============================
#	Code Testing & Fixing
# ===============================

test:
	go test -v ./...

test-race:
	go test -race ./...

test-coverage:
	go test -coverage=coverage.out ./...

vet:
	go vet ./...


# ===============================
#	Code Formatting & Linting
# ===============================

fmt:
	go fmt ./...

gofmt:
	gofmt -l -w $(dir)

# ===============================
#	Database Operations
# ===============================

# ===============================
#	SQLC
# ===============================

sqlc:
	sqlc generate

# ===============================
#	Database Migration
# ===============================

migrate-create:
	migrate create -ext sql -dir ./internal/infrastructure/database/postgres/migrations/ -seq $(name)

migrate-up:
	migrate -path ./internal/infrastructure/database/postgres/migrations/ -database "$(DB_URL)" up

migrate-down:
	migrate -path ./internal/infrastructure/database/postgres/migrations/ -database "$(DB_URL)" down

migrate-force-version:
	migrate -path ./internal/infrastructure/database/postgres/migrations/ -database "$(DB_URL)" force $(version)

app-migrate-up:
	go run ./cmd/migrate up

app-migrate-down:
	go run ./cmd/migrate down

app-migrate-version:
	go run ./cmd/migrate version

app-migrate-version-force:
	go run ./cmd/migrate force $(version)

# ===============================
#	Security & Vulnarability
# ===============================

gosec:
	gosec ./...

govulncheck:
	govulncheck ./...