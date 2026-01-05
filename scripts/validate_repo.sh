#!/usr/bin/env bash
set -euo pipefail

# 1. Ensure vision.md exists
test -f vision.md || { echo "vision.md missing"; exit 2; }

# 2. Ensure proto exists
test -f api/pulsaar.proto || { echo "api/pulsaar.proto missing"; exit 2; }

# 3. Generate proto go stubs if protoc present
if command -v protoc >/dev/null 2>&1; then
  protoc --go_out=. --go-grpc_out=. api/pulsaar.proto
fi

# 4. Check that progress.md contains 'Next steps'
grep -q "Next steps" progress.md || { echo "progress.md missing Next steps"; exit 2; }

# 5. Basic lint for rules.md presence
test -s rules.md || { echo "rules.md empty"; exit 2; }

# 6. Run unit tests if present
if [ -d ./cmd ] || [ -d ./pkg ]; then
  if command -v go >/dev/null 2>&1; then
    go test ./... || { echo "go tests failed"; exit 3; }
  fi
fi

echo "VALIDATION_OK"
