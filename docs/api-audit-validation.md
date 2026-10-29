# API audit validation

The binding and service fixes were checked locally and on the requested host
`10.0.0.85` (Linux x86_64, libvirt 10.2.0, QEMU 10.2.0).

## Compatibility and generation

- All 568 raw function bindings remain present, matching the four vendored
  libvirt 12.6.0 XML documents. The 1,093 enum constants remain generated.
- Object-like macro values and the native structures used by high-level
  wrappers now come from the same XML. Application statistics models,
  ownership, error handling, and typed-parameter validation remain Go code.
- No existing exported function/method signature was removed or changed.
  `Domain.HasManagedSaveImage(uint32) (bool, error)` is additive.
- `DomainStats` fields, counter widths, native units, missing-value flags,
  negative-sentinel handling, and sparse device indices remain unchanged.
  It shares the native parameter decoder with job and tuning parameters.
- Native typed-parameter Add/Free/Clear helpers are preferred. The existing ABI
  path remains usable when helpers from libvirt 1.0.2 are absent or only partly
  available. Both paths preserve duplicate fields, strings and numeric limits.
- The service selects the binding's pinned Git submodule through `go.mod`.
  Run `git submodule update --init libvirt-go` before building a fresh checkout.
- Explicit unencrypted TPM directory sources from domain XML are honored.
  Legacy default locations remain supported for libvirt before 10.10, which
  does not publish the source in XML. Encrypted/file sources are unsupported;
  relative and root-directory sources are rejected.

## Local checks

Passed:

- `CGO_ENABLED=0 go test ./...` and `CGO_ENABLED=0 go vet ./...` in the binding.
- Regeneration and byte-for-byte generated-file consistency.
- Linux amd64 and arm64 binding builds with cgo disabled.
- C compiler comparison of the generated native structure sizes and all field
  offsets against the official libvirt 12.6.0 headers, including typed-parameter
  union storage. Set `LIBVIRT_HEADER_DIR` to reproduce this optional check.
- Minimal actual shared-library regression: a missing main-library symbol
  stays unavailable after extension loading, and RawAPI returns the compatibility
  error instead of calling a nil function.
- Actual shared-library scheduler contract fixture: obtain the parameter count
  from `virDomainGetSchedulerType`, then fetch the parameters.
- Linux cross-compilation of all service package tests and the HTTP E2E binary,
  plus `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go vet ./...` in the service.

## Checks on 10.0.0.85

Passed:

- Real native parameter allocation, decoding, and cleanup with all seven types,
  integer limits, infinity, Unicode strings and duplicate fields. Native,
  partial-helper and pre-1.0.2 compatibility paths all passed.
- Real QEMU domain fixture: paused startup/resume, scheduler read/write,
  structured state/CPU/memory/vCPU/block statistics, and lifecycle callbacks.
- Real older-library probe: `HasSymbol("virDomainAnnounceInterface")` is false;
  its RawAPI method returns `ErrSymbolUnavailable` (introduced in 12.5.0).
- Service, adapter, configurator, image, and filesystem package suites. The
  explicit TPM-directory filesystem regression is included in the service suite.
- All route tests, using separate processes for the test that initializes the
  message bus and the remaining tests.
- Remaining virt package tests after excluding the unavailable test-driver
  scenario described below.
- HTTP `TestDomainMetricsE2E`: stopped/running samples, native counters,
  configurable intervals, keepalive, disconnect and undefined-domain handling.
- HTTP `TestNativeBackupPowerStatesE2E`: shut-off, running and paused backups.
- HTTP `TestColdBackupE2E`: BIOS boot/data recovery and UEFI/TPM state recovery.
- HTTP `TestBackupRejectsManagedSaveE2E`: saved state survives redefinition,
  native saved-image queries return true, and backup returns HTTP 409. Its
  cleanup uses an independent bounded context because `testing.T.Context` is
  canceled before cleanup callbacks run.

VM and storage-pool UUID inventories were compared before and after testing:
6 existing VMs and 2 existing pools, with no additions or removals. All temporary
VMs and pools created by these scenarios were cleaned up. Test binaries and logs
are under `/DATA/zvm-api-audit.qto5Sd` on the host.

## Environment limits and suite isolation

The host has no `virttestd` socket. `TestIntegrationTestDriver` and
`TestBridgeNamesFromActiveLibvirtConfiguration` therefore cannot use
`test:///default` there. The requested QEMU workflows were exercised against
`qemu:///session` instead; no system daemon was installed or reconfigured.

The unmodified route suite, run in one process, exposes existing test interference:
`TestPCIPassthroughAPIIsDisabled` starts a reconnecting message-bus client and later
XML mapping tests call `goleak.VerifyNone`. Running that first test separately,
then the suite with `-test.skip '^TestPCIPassthroughAPIIsDisabled$'`, passes both
sets without changing production message-bus behavior or suppressing leak checks.
