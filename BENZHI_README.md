# BENZHI build notes

The repository is self-contained and uses the Go standard library. Run with Go 1.23.12 and `GOTOOLCHAIN=local`. The service persists state to `FORESTPULSE_DATA_PATH`; tests and smoke commands use temporary directories and do not depend on host data.

Docker build:

```text
docker build -f benzhi.Dockerfile -t forestpulse:local .
```

The image runs as an unprivileged user and stores data below `/var/lib/forestpulse`.

