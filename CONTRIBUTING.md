# Contributing

Thanks for considering a contribution! GoLogX is small on purpose — the bar for new surface area is high, the bar for tightening what's already there is low.

## Setup

```bash
git clone https://github.com/AyoubTadlaoui/GoLogX.git
cd GoLogX
make check       # gofmt + vet + race tests
```

## Workflow

1. Open an issue if you're proposing a new feature, so we can agree on scope before code is written.
2. Fork & branch off `main`.
3. Make the change. Keep the diff focused.
4. Run `make check` (and `make lint` if you touched the API).
5. Open a PR. CI must pass before merge.

## What we love

- A bug fix paired with a regression test.
- A performance improvement backed by `go test -bench` numbers (before / after).
- Doc and README improvements that flatten the on-ramp.
- Replacing a hand-rolled helper with a stdlib one (with rationale).

## What slows things down

- Adding a runtime dependency. **Stdlib first.** GoLogX has zero external deps and intends to keep it that way.
- Reformatting unrelated files in the same diff.
- Code without tests; tests without assertions.
- "Refactor for elegance" without a measurable benefit.

## Style

- `make fmt`, `make vet`, `make lint` must pass.
- Doc comments on every exported symbol, starting with the symbol name.
- Comments explain *why*, not *what*.
- Hot-path code should justify allocations — `go test -benchmem` will tell you.

## Releasing (maintainers)

1. Land all changes for the release on `main` and ensure CI is green.
2. Update [CHANGELOG.md](CHANGELOG.md) — promote `[Unreleased]` items under a new dated heading.
3. Tag: `git tag -a vX.Y.Z -m "vX.Y.Z"` and push: `git push origin vX.Y.Z`.
4. The `release.yml` workflow picks up the tag and runs goreleaser, which builds the binaries and creates the GitHub Release.
