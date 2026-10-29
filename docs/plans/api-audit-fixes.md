# API Audit Fixes Implementation Plan

**Goal:** Fix the audit findings and commit each independent change separately in the binding and service repositories.

**Architecture:** Keep high-level ownership and Go conversion logic handwritten. Generate upstream constants and native layouts from the vendored XML, and use native libvirt helpers where their contracts apply.

**Tech Stack:** Go, purego, libvirt XML metadata, native integration fixtures.

## Commit sequence and acceptance checks

1. Preserve missing main-library symbols during extension binding (`loader_unix.go`). Compile a minimal shared-library fixture and verify public RawAPI calls report `ErrSymbolUnavailable` without panicking.
2. Query scheduler parameter counts using `virDomainGetSchedulerType` (`typed_params.go`). Cover the distinct scheduler contract and extend the native test-driver scenario.
3. Generate object-like macro constants (`internal/generator`, `libvirt_api.gen.go`). Replace handwritten job names, statistics keys, UUID and array limits. Verify string escaping, numeric expressions, aliases and deterministic regeneration.
4. Generate the concrete native layouts used by the binding from XML, including the typed-parameter union. Verify field offsets and sizes against the C ABI and cross-build supported production architectures.
5. Use native typed-parameter add/free helpers and retain Go input validation. Verify mixed numeric/string parameters, failure cleanup and native round trips.
6. Add `Domain.HasManagedSaveImage` and use it in the service, preserving the service's UUID lookup semantics. Validate stopped/saved workflows through existing tests.
7. Remove inferred TPM state locations. Resolve state from explicit libvirt domain XML, adapt cold-backup fixtures and recovery documentation, and reject ambiguous legacy paths before producing a purported complete backup.

## Validation

Run targeted checks per commit, then `CGO_ENABLED=0 go test ./...`, vet, regeneration consistency, and Linux cross-builds for the binding. Run service tests using a temporary Go workspace that selects the local binding, plus compile the provisioned-host E2E suite. Run real native libvirt integration when the runtime is available; record unavailable environment-dependent checks explicitly. Keep the two repositories clean and record their separate commit IDs.
