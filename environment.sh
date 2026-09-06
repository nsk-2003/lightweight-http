#!/usr/bin/env bash
# Purpose: Configure the local development environment for the lightweight-http project.
# Source it, do not execute it:  source ./environment.sh && go build ./...
#
# Deliberately does not use `set -e`: this file is sourced into an interactive shell, where
# an aborting option would terminate the user's session on the first failing command.

# Resolve this file's location under both bash and zsh.
if [ -n "${ZSH_VERSION:-}" ]; then
    eval '_lwhttp_src="${(%):-%x}"'
else
    _lwhttp_src="${BASH_SOURCE[0]:-$0}"
fi

LWHTTP_ROOT="$(cd "$(dirname "$_lwhttp_src")" && pwd)"
unset _lwhttp_src

export LWHTTP_ROOT
export LWHTTP_CMD="$LWHTTP_ROOT/cmd"
export LWHTTP_PKG="$LWHTTP_ROOT/pkg"
export LWHTTP_TEST="$LWHTTP_ROOT/test"
export LWHTTP_BUILD="$LWHTTP_ROOT/build"

export LWHTTP_PORT="${LWHTTP_PORT:-8080}"
export LWHTTP_LOG_LEVEL="${LWHTTP_LOG_LEVEL:-info}"
export LWHTTP_DEBUG="${LWHTTP_DEBUG:-false}"

export CGO_ENABLED="${CGO_ENABLED:-0}"

# Machine-specific overrides live here and are not committed. See docs/devenv.md.
if [ -f "$LWHTTP_ROOT/environment.local.sh" ]; then
    . "$LWHTTP_ROOT/environment.local.sh"
fi

if ! command -v go >/dev/null 2>&1; then
    echo "ERROR: go toolchain not found on PATH. See docs/devenv.md" >&2
else
    echo "Environment configured for lightweight-http"
    echo "  LWHTTP_ROOT=$LWHTTP_ROOT"
    echo "  LWHTTP_PORT=$LWHTTP_PORT"
    echo "  LWHTTP_LOG_LEVEL=$LWHTTP_LOG_LEVEL"
    echo "  go=$(go version | awk '{print $3}')"
fi
