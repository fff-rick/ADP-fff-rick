$ErrorActionPreference = "Stop"

Write-Host "Running Phase 6 targeted package tests..."
go test ./internal/api ./internal/scheduler ./internal/analyzer

Write-Host "Running Phase 6 integration acceptance tests..."
go test ./tests/integration/...

Write-Host "Phase 6 acceptance checks completed."
