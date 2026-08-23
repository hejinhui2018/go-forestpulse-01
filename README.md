# ForestPulse

ForestPulse is a Go service for durable environmental telemetry ingestion. It keeps station identity, ordered readings, batch outcomes, and station cursors in an atomic local snapshot suitable for edge deployments.

## Requirements

- Go 1.23.12
- `GOTOOLCHAIN=local`
- `CGO_ENABLED=0` for release builds

## Run

```text
go run ./cmd/forestpulse-server
```

Configuration uses `FORESTPULSE_HTTP_ADDR`, `FORESTPULSE_DATA_PATH`, `FORESTPULSE_RECOVERY_EVERY`, `FORESTPULSE_REQUEST_TIMEOUT`, and `FORESTPULSE_MAX_BATCH`.

## Smoke

```text
go run ./cmd/forestpulse-smoke
```

The smoke executable creates an isolated data directory, registers a station, submits a two-reading batch through HTTP, and confirms that the persisted cursor reaches sequence 2.

## Checks

Run these separately:

```text
go test ./... -count=1
go vet ./...
go build ./...
```

