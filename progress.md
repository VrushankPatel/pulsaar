# progress.md

## Project: Pulsaar

### Current state
- vision.md created
- api/pulsaar.proto created

### Last commit summary
- <commit-hash> - created initial vision and proto

### Decisions log
- Default MVP connection: kubectl port-forward or apiserver proxy
- mTLS production requirement via cert-manager

### Known issues
- Security team adoption risk
- Certificate management not yet implemented
- Sidecar injection design pending

### Next steps
1. Implement Pulsaar agent scaffold in Go with proto stubs
2. Implement ListDirectory and ReadFile handlers with path allowlist and size limits
3. Create CLI `pulsaar explore` that uses kubectl port-forward and TLS as MVP
4. Add unit tests for path sanitization and allowlist enforcement
5. Add validation script validate/reconcile files

### Stop conditions met
- None
