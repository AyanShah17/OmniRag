# Branching Workflow

`dev` is the continuous development branch. Errors, mistakes, fixes, refactors,
and new features are developed there.

`pre-prod` contains the latest candidate from `dev` while it is being reviewed
and tested for functionality, regressions, and release readiness.

`prod` contains the latest verified working prototype. Promote changes from
`pre-prod` only after review and functional verification are complete.

`main` is retained as the repository baseline and should not receive direct
development commits. Keep it aligned with the verified `prod` baseline when
the repository owner chooses to update it.

## Promotion Flow

```text
feature work -> dev -> pre-prod -> prod
```

Before each promotion:

1. Merge continuous work from `dev` into `pre-prod` for review.
2. Run Python tests, Go tests, `go vet`, and the frontend build on `pre-prod`.
3. Review functionality, regressions, secrets, and generated artifacts.
4. Promote the verified candidate from `pre-prod` into `prod`.
5. Keep `prod` as the known working prototype while new work continues in `dev`.

The working branch for ongoing agent development is `dev`.
