<!-- Use a plain sentence for the PR title. Every commit must use a Conventional Commit subject. -->

## What & why

<!-- What does this change and why? Link issues (e.g. "Closes #123"). -->

## Checklist

- [ ] `mise run check` is green (`go build`, `go vet`, `gofmt -l .` empty, `go test -race ./...`)
- [ ] New behavior has tests; ported behavior reuses the upstream `.sql` fixtures
- [ ] Ported 1:1 from `.reference/sqlglot-v30.12.0/` where applicable (cite reference lines for any divergence)
- [ ] Any intentional difference from upstream is recorded in `DEVIATIONS.md` with a rationale
- [ ] PR title is a plain sentence; commit subjects are Conventional Commits
- [ ] Did **not** hand-edit `CHANGELOG.md` or version tags (release-please owns those)
