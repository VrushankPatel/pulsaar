# progress.md

## Project: Pulsaar

### Current state
- vision.md created
- api/pulsaar.proto created
- rules.md created
- Pulsaar agent scaffold implemented in Go with proto stubs
- ListDirectory and ReadFile handlers implemented with path allowlist and 1MB size limits
- Unit tests added for path sanitization and allowlist enforcement

### Last commit summary
- Implement ListDirectory and ReadFile handlers with path allowlist and size limits

### Decisions log
- Default MVP connection: kubectl port-forward or apiserver proxy
- mTLS production requirement via cert-manager
- Max read size set to 1MB for MVP

### Known issues
- Security team adoption risk
- Certificate management not yet implemented
- Sidecar injection design pending

### Next steps
1. Create CLI `pulsaar explore` that uses kubectl port-forward and TLS as MVP
2. Add validation script validate/reconcile files

### Stop conditions met
- None
