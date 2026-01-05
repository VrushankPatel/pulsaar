# progress.md

## Project: Pulsaar

### Current state
- vision.md created
- api/pulsaar.proto created
- rules.md created
- Pulsaar agent scaffold implemented in Go with proto stubs
- Agent serves with TLS using self-signed certificate for MVP
- ListDirectory, ReadFile, Stat, and StreamFile handlers implemented with path allowlist and 1MB size limits
- Unit tests added for path sanitization and allowlist enforcement
- CLI `pulsaar explore` implemented with kubectl port-forward and TLS connection
- CLI `pulsaar read` implemented with kubectl port-forward and TLS connection
- CLI `pulsaar stream` implemented with kubectl port-forward and TLS connection
- CLI `pulsaar stat` implemented with kubectl port-forward and TLS connection
- CLI and agent binaries built
- End-to-end CLI functionality tested with integration test
- Audit logs implemented for all operations (ListDirectory, ReadFile, Stat, StreamFile) to stdout
- Optional aggregator for audit logs implemented (HTTP POST to configurable URL via PULSAAR_AUDIT_AGGREGATOR_URL env var)


### Last commit summary
- Implement optional audit log aggregator

### Decisions log
- Default MVP connection: kubectl port-forward or apiserver proxy
- mTLS production requirement via cert-manager
- Max read size set to 1MB for MVP
- Used exec.Command for kubectl port-forward in CLI for MVP simplicity
- Optional audit aggregator sends structured JSON logs via HTTP POST

### Known issues
- Security team adoption risk
- Certificate management not yet implemented
- Sidecar injection design pending

### Next steps
- Implement certificate management for production mTLS

### Stop conditions met
- None
