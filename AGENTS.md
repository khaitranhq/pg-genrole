# AGENTS.md

## Enforcement

Rules that must always be followed. No exceptions.

### 1. Go Validation

After any Go change, run:

```bash
go build -o /dev/null ./...
golangci-lint run ./...
```

Fix all reported issues. Mark false positives with `//nolint` + justification comment.

### 2. Go Formatting

Format all touched Go files before committing, in order:

```bash
gofmt -w .
gofumpt -w .
goimports -w .
```
