#!/bin/bash
# SessionStart hook: point git at the tracked hooks so the pre-push guard
# works immediately in a fresh checkout.
set -euo pipefail

cd "${CLAUDE_PROJECT_DIR:-.}"

git config core.hooksPath .githooks

# Only worth it in remote containers; local sessions have a warm module cache.
if [ "${CLAUDE_CODE_REMOTE:-}" = "true" ]; then
	go mod download
fi
