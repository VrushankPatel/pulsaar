# progress.md

## Project: Pulsaar

### Current state
- vision.md created
- api/pulsaar.proto created
- rules.md created
- Pulsaar agent scaffold implemented in Go with proto stubs
- Agent serves with TLS using self-signed certificate for MVP
- ListDirectory and ReadFile handlers implemented with path allowlist and 1MB size limits
- Unit tests added for path sanitization and allowlist enforcement
- CLI `pulsaar explore` implemented with kubectl port-forward and TLS connection
- Repository has uncommitted changes: modified cmd/agent/main.go, added cmd/cli/main.go, updated go.mod/go.sum

### Last commit summary
- Create CLI `pulsaar explore` that uses kubectl port-forward and TLS as MVP

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
1. Build CLI and agent binaries
2. Test end-to-end CLI functionality

### Stop conditions met
- None
