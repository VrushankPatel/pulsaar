# Next Steps for Pulsaar Development

## Overview
This document outlines the key areas that need attention for continued development of **Pulsaar**. It captures required tooling, pending enhancements, refactoring opportunities, and operational tasks to keep the project healthy and production‑ready.

## Development Prerequisites
1. **Go toolchain** – Go 1.25+ (the `go.mod` declares `go 1.25.0`). Install from https://golang.org/dl/.
2. **Protocol Buffers** – `protoc` and the `protoc-gen-go` and `protoc-gen-go-grpc` plugins. The repository includes a `protoc` binary under the repo root for convenience.
3. **Kubernetes tooling** – `kubectl` for local testing, `kind` (or a real cluster) to spin up a test cluster, and appropriate kubeconfig.
4. **Docker** – for building and publishing container images (see the Dockerfiles in the repo).
5. **Git** – standard Git workflow; the repository uses a `master` branch.
6. **CI/CD** – GitHub Actions are defined in `.github/workflows/ci.yml`. Ensure the GitHub runner has the above tools installed.

## Pending Enhancements & Refactors
| Area | Description | Suggested Implementation |
|------|-------------|--------------------------|
| **Deny‑list UI** | Expose the deny‑list via the CLI (`pulsaar config set‑denylist …`) and optionally via a ConfigMap. | Add new Cobra sub‑command that updates the `PULSAAR_DENIED_PATHS` env var or writes to a ConfigMap. |
| **ConfigMap Integration** | Currently only the allow‑list reads from a ConfigMap. Add parallel support for a deny‑list ConfigMap. | Extend `initConfiguredDeniedPaths()` to read `pulsaar-denylist` ConfigMap similar to allow‑list. |
| **Rate‑limit Configuration** | The rate limit (10 ops/sec) is hard‑coded. Make it configurable via env var or ConfigMap. | Introduce `PULSAAR_RATE_LIMIT` and adjust `getLimiterForIP`. |
| **Unit Test Coverage** | Tests cover the RPC methods but lack coverage for path‑allow/deny logic. | Add table‑driven tests for `isPathAllowed` covering allow‑list, deny‑list, and edge cases. |
| **Metrics Export** | Prometheus metrics are exposed, but no custom metrics for audit‑log failures. | Add a counter metric `audit_log_errors_total`. |
| **Documentation** | The README is minimal and Vision.md is not linked from the docs. | Generate a `docs/` site using Hugo or MkDocs and link to `vision.md`, `progress.md`, and the new `next-steps.md`. |
| **Error Messages** | Some errors expose internal file paths. Replace with user‑friendly messages while logging details. | Refactor error handling to wrap internal errors using `status.Errorf`. |
| **CI Improvements** |
   - Run `golangci-lint` with the repository’s lint config. |
   - Add a step that validates the built Docker images (`docker build` & `docker run --rm`). |
| **Release Automation** | Automate release tagging, binary signing, and checksum publishing. |
   - Use GitHub Release Action to upload `agent`, `cli`, `aggregator`, and `webhook` binaries. |
|

## Refactoring Opportunities
- **Separate Configuration Loading**: Move all env‑var and ConfigMap loading into a dedicated `config` package. This reduces duplication between the agent and CLI.
- **Logging Abstraction**: Replace ad‑hoc `log.Printf` calls with a structured logger (e.g., `zap` or `logrus`) to embed timestamps and fields.
- **Common Path Helper**: Extract the deny‑list/allow‑list check into a reusable function that can be used by both client and server side tooling.

## Operational Tasks
- **Update CI Pipeline** to run `./scripts/validate_repo.sh` on every PR.
- **Add a GitHub Issue Template** for feature requests and bug reports.
- **Create a CONTRIBUTING checklist** that references this `next-steps.md`.
- **Publish Helm chart** to a chart repository (e.g., `artifacthub.io`).
- **Set up monitoring alerts** for high rate‑limit rejections and audit‑log failures.

## Getting Started Checklist
1. Clone the repo and `cd pulsaar`.
2. Run `go mod tidy`.
3. Generate protobuf stubs: `./protoc` (or `protoc --go_out=. --go-grpc_out=. api/pulsaar.proto`).
4. Build binaries: `go build ./cmd/agent && go build ./cmd/cli && go build ./cmd/webhook`.
5. Run unit tests: `go test ./...`.
6. Spin up a Kind cluster and test end‑to‑end via `pulsaar explore`.
7. Review and address items in the table above.

---
*This document is a living roadmap; keep it in sync with the project backlog and GitHub issues.*
