$ErrorActionPreference = "Stop"

Write-Host "Running Phase 6 targeted package tests..."
go test ./internal/interfaces/http ./internal/infrastructure/scheduler ./internal/application/analyzer

Write-Host "Running Phase 6 integration acceptance tests..."
go test ./tests/integration/...

Write-Host "Phase 6 acceptance checks completed."
