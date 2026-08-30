#!/usr/bin/env bash
set -euo pipefail

target="${1:?deployment target is required}"
if [[ "${target}" != "pre-prod" && "${target}" != "prod" ]]; then
  echo "Unsupported deployment target: ${target}" >&2
  exit 64
fi

if [[ -z "${DEPLOY_COMMAND:-}" ]]; then
  echo "DEPLOY_COMMAND is not configured for ${target}." >&2
  echo "Set it in the restricted ${target}-deploy CircleCI context." >&2
  exit 78
fi

echo "Executing the approved ${target} deployment command."
bash -lc "${DEPLOY_COMMAND}"
