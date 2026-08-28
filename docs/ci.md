# Continuous integration

The GitHub Actions setup combines patterns from
[`digitalocean/go-libvirt`](https://github.com/digitalocean/go-libvirt) and
[`libvirt/libvirt-go-module`](https://gitlab.com/libvirt/libvirt-go-module/):
multiple Linux/libvirt environments, generated-code checks, race tests, real
daemon fixtures, and separate security analysis.

## CI workflow

`.github/workflows/ci.yml` runs on pushes, pull requests, version tags, manual
dispatches, and a weekly schedule.

- **Quality** runs formatting, generated-source and `go mod tidy` checks,
  cgo-free build/test/vet, race tests, Linux arm64 cross-build, actionlint, and
  coverage upload.
- **Synthetic integration** runs `test:///default` on Ubuntu 22.04 and 24.04,
  providing older-runtime compatibility coverage.
- **Real integration** starts an isolated TCP libvirt/QEMU daemon on an
  ephemeral Ubuntu 24.04 runner and runs the mutating fixtures with guest
  lifecycle callbacks enabled.
- **Govulncheck** scans all Go packages.

The real job uses `ci/libvirtd.conf`, listens only on runner-localhost, and must
never be copied to a production host.

## Security workflow

`.github/workflows/codeql.yml` runs CodeQL for pushes, pull requests, manual
runs, and a weekly schedule. Actions are pinned to immutable commit SHAs;
Dependabot tracks both Go modules and GitHub Actions.

## Local commands

```sh
make ci
make coverage
make test-integration
```

`make test-real` requires the explicit environment gates documented in
`testing.md` and should run only on a disposable Linux host or VM.

## Release gating

Version tags trigger the full CI workflow. This repository is a Go library, so
there is no binary deployment step; a tag is publishable only after all required
CI and CodeQL checks pass. Configure branch protection for `main` to require the
quality, synthetic integration, real integration, and vulnerability jobs.
