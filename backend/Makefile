-include ../.env

run:
	go run cmd/main.go

# Build a self-contained Windows .exe with the frontend AND client tools embedded.
# The exe extracts tools to assets/tools/win-x64/ beside itself on first launch.
# Run from the backend/ directory: make build-windows
build-windows:
	cd ../frontend && pnpm build
	rm -rf ui/build && cp -r ../frontend/dist ui/build
	cp -r ../assets/tools/win-x64/. embedded_tools/win-x64/
	GOOS=windows GOARCH=amd64 go build -o ../databasus.exe ./cmd/
	rm -rf embedded_tools/win-x64 && mkdir -p embedded_tools/win-x64 && touch embedded_tools/win-x64/.gitkeep
	cd .. && rm -f databasus-windows-x64.zip && zip databasus-windows-x64.zip databasus.exe install-service.bat install-service.ps1 update.ps1
	@echo "Done: ../databasus-windows-x64.zip"

# cleanup_test_db resets the metadata DB AND each test PG container
# (drops leftover roles/databases and recreates testdb) so containers
# stay usable across runs - tests that REVOKE on public or create
# per-run roles would otherwise poison subsequent pg_dump calls.
test:
	go run ./cmd/cleanup_test_db
	goose -dir ./migrations postgres "$(GOOSE_TEST_DBSTRING)" up
	go test -p=1 -count=1 -failfast -timeout 15m ./internal/...

lint:
	golangci-lint fmt ./cmd/... ./internal/... && golangci-lint run ./cmd/... ./internal/...

migration-create:
	goose -dir ./migrations create $(name) sql

migration-up:
	goose -dir ./migrations postgres "$(GOOSE_DBSTRING)" up

migration-up-test:
	goose -dir ./migrations postgres "$(GOOSE_TEST_DBSTRING)" up

migration-down:
	goose -dir ./migrations postgres "$(GOOSE_DBSTRING)" down

swagger:
	swag init -g ./cmd/main.go -o swagger
