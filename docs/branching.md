# Branching Workflow

`dev` is the active development branch. New implementation work, fixes, and
tests start there.

`pre-prod` is the integration and release-candidate branch. Promote changes
from `dev` with a pull request after the full test suite and frontend build
pass.

`prod` contains production-ready code. Promote only validated changes from
`pre-prod` with a pull request and deployment approval.

`main` is retained as the repository baseline and should not receive direct
development commits. Keep it aligned with the approved production baseline
when the repository owner chooses to update it.

## Promotion Flow

```text
feature work -> dev -> pre-prod -> prod
```

Before each promotion:

1. Run Python tests, Go tests, `go vet`, and the frontend build.
2. Review the diff and confirm no secrets or generated artifacts are included.
3. Merge through a pull request into the next branch.
4. Deploy only from `prod`.

The working branch for ongoing agent development is `dev`.
