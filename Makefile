include .env
export

help:
	@echo "Available Commands:\n"

	@echo "Help Commands:"
	@echo "  --help					Show make commands"
	@echo "  --go-help					Show a named go command\n"

	@echo "Dependency Management:"
	@echo "  init						Initialize go module"
	@echo "  get						Add dependency to the module"
	@echo "  update					Update the the named dependency in the module to its latest version"
	@echo "  install					Install a dependency to the system"
	@echo "  download					Download required modules and dependencies"
	@echo "  fix						Update and fix obsolete syntax used in go files"
	@echo "  tidy						Add missing and clean unused dependencies"
	@echo "  list						List all packages in the module"
	@echo "  list-json					List all packages in the module in json format\n"

	@echo "Cache Management:"
	@echo "  clean-cache					Delete cached go data"
	@echo "  clean-test-cache				Delete cached go test data\n"

	@echo "API Documentation"
	@echo "  swagger					Document API in swagger\n"

	@echo "Code Testing & Fixing:"
	@echo "  test						Perform all unit tests recursively"
	@echo "  test-race					Perform race condition test on all files recursively"
	@echo "  test-coverage					Perform test coverage and write results to coverage.out file"
	@echo "  vet						Scan erronous code\n"

	@echo "Code Formatting & Linting:"
	@echo "  fmt						Format all go source files"
	@echo "  gofmt						Format all go source files in the named directory\n"
	

	@echo "SQLC:"
	@echo "  sqlc						Generate SQLC code and queries\n"

	@echo "Database Migrations:"
	@echo "  migrate-create				Create SQL migration files"
	@echo "  migrate-up					Apply migrations"
	@echo "  migrate-down					Drop migrations"
	@echo "  migrate-force-version				Force migration version to the named value"
	@echo "  app-migrate-up				Apply migrations using migrate binary"
	@echo "  app-migrate-down				Drop migrations using migrate binary"
	@echo "  app-migrate-version-force			Force migration version to the named value using migrate binary\n"

	@echo "Security & Vulnarability Scan:"
	@echo "  gosec						Run security scan on go source file"
	@echo "  govulncheck					Run vulnarability check on go source file\n"

--help:
	--help

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