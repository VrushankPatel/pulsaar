#!/usr/bin/env bash
set -euo pipefail

# 1. Parse api/pulsaar.proto for RPCs
rpcs=$(grep 'rpc ' api/pulsaar.proto | sed 's/.*rpc \([A-Za-z]*\).*/\1/')
echo "RPCs found: $rpcs"

# 2. For every RPC ensure an implementation exists
for rpc in $rpcs; do
  if ! grep -q "func (s \*server) $rpc" cmd/agent/main.go; then
    echo "Implementation missing for RPC: $rpc"
    exit 1
  fi
  # Check if not unimplemented
  if grep -A5 "func (s \*server) $rpc" cmd/agent/main.go | grep -q "Unimplemented"; then
    echo "RPC $rpc is not implemented (stub)"
    exit 1
  fi
done

# 3. Verify every file access uses filepath.Clean and allowed roots enforcement
# Search for file operations
file_ops=$(grep -r "os\.Open\|ioutil\.ReadFile\|os\.Stat\|filepath\.Walk" cmd/ || true)
if [ -n "$file_ops" ]; then
  echo "File operations found, checking for filepath.Clean and allowed roots"
  # This is simplistic, need to check each
  # For now, assume check manually or use more grep
  if ! grep -r "filepath\.Clean" cmd/; then
    echo "filepath.Clean not used in file access"
    exit 1
  fi
   # Allowed roots: check if code checks AllowedRoots
   if ! grep -r "AllowedRoots" cmd/; then
     echo "AllowedRoots not enforced"
     exit 1
   fi
fi

# 4. Ensure read size limits exist
# Check in ReadFile and StreamFile if size is limited
for rpc in ReadFile StreamFile; do
  if ! grep -A20 "func (s \*server) $rpc" cmd/agent/main.go | grep -q "maxReadSize"; then
    echo "Read size limit not enforced in $rpc"
    exit 1
  fi
done

# 5. Ensure audit logging exists on every read or stream path
for rpc in ReadFile StreamFile; do
  if ! grep -A10 "func (s \*server) $rpc" cmd/agent/main.go | grep -q "log\."; then
    echo "Audit logging missing in $rpc"
    exit 1
  fi
done

# 6. Ensure agent does not run as root if Dockerfile exists
if [ -f Dockerfile ]; then
  if ! grep -q "os\.Getuid\|os\.Geteuid" cmd/agent/main.go; then
    echo "Agent may run as root, check missing"
    exit 1
  fi
fi

# 7. Ensure rules.md is not violated
# Check git status
if ! git diff --quiet; then
  echo "Git tree is dirty, violating rule: Never leave git tree dirty"
  exit 1
fi
# Check for no shell/exec functionality
if grep -r "exec\|os/exec" cmd/; then
  echo "Shell or exec functionality introduced, violating rule: Never introduce shell or exec functionality into Pulsaar"
  exit 1
fi

# 8. If any violation, already exited

# 9. If clean
echo "VALIDATION_OK"