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
- CLI and agent binaries built
- End-to-end CLI functionality tested with integration test
- Audit logs implemented for ReadFile and StreamFile operations to stdout


### Last commit summary
- Add stream file command to CLI

### Decisions log
- Default MVP connection: kubectl port-forward or apiserver proxy
- mTLS production requirement via cert-manager
- Max read size set to 1MB for MVP
- Used exec.Command for kubectl port-forward in CLI for MVP simplicity

### Known issues
- Security team adoption risk
- Certificate management not yet implemented
- Sidecar injection design pending

### Next steps
- Implement CLI `pulsaar stat` command
- Add audit logs for ListDirectory and Stat operations
- Implement optional aggregator for audit logs

### Stop conditions met
- None
