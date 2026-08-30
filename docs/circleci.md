# CircleCI Setup

The repository uses CircleCI with GitHub as its VCS provider. The configuration
is in `.circleci/config.yml` and uses CircleCI machine executors, so application
Dockerfiles are not required.

## Branch behavior

- Every branch runs the `quality` job: Python tests, Go tests, `go vet`, Go build,
  and frontend build.
- `dev` is continuous development and receives validation only.
- `pre-prod` pauses after successful validation for manual approval, then runs
  the deployment command from the restricted `pre-prod-deploy` context.
- `prod` pauses after successful validation for manual approval, then runs the
  deployment command from the restricted `prod-deploy` context.

## GitHub connection

1. Sign in to CircleCI using the GitHub organization that owns this repository.
2. Follow the project and select `AyanShah17/OmniRag`.
3. Select the existing `.circleci/config.yml` configuration.
4. Add the CircleCI checks to GitHub branch protection rules for `dev`,
   `pre-prod`, and `prod`.

## Restricted deployment contexts

Create two CircleCI contexts:

- `pre-prod-deploy`
- `prod-deploy`

Add `DEPLOY_COMMAND` to each context. Restrict each context to this project and
to its matching branch. Restrict `prod-deploy` to the production maintainers'
GitHub team. Do not put cloud credentials in `.circleci/config.yml`; add them
to the appropriate restricted context and reference them from the deployment
command.

The repository cannot choose the deployment command because no hosting target
is defined. `scripts/circleci/deploy.sh` provides the controlled hook and fails
closed when a context has not been configured.
