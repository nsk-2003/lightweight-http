# Package Design

This document describes the intended module layout and dependencies for the lightweight
HTTP framework. It is the **target** design; update it after each phase so it matches the
real import graph.

## Target Module Dependency Diagram

```mermaid
flowchart TB
  subgraph app [Layer 4 - Application]
    cmdMain[cmd/server]
    examplesPkg[examples]
  end
  subgraph composition [Layer 3 - Composition]
    middlewarePkg[pkg/middleware]
    observabilityPkg[pkg/observability]
  end
  subgraph core [Layer 2 - Core]
    routerPkg[pkg/router]
    diPkg[pkg/di]
  end
  subgraph foundation [Layer 1 - Foundation]
    errorsPkg[pkg/errors]
  end
  subgraph stdlib [Standard Library]
    nethttp[net/http]
    neturl[net/url]
    ctx[context]
    syncp[sync]
    slog[log/slog]
  end

  cmdMain --> routerPkg
  cmdMain --> middlewarePkg
  cmdMain --> observabilityPkg
  cmdMain --> diPkg
  examplesPkg --> routerPkg
  examplesPkg --> middlewarePkg
  examplesPkg --> diPkg
  middlewarePkg --> routerPkg
  middlewarePkg --> errorsPkg
  observabilityPkg --> errorsPkg
  routerPkg --> errorsPkg
  diPkg --> errorsPkg
  routerPkg --> nethttp
  routerPkg --> neturl
  routerPkg --> ctx
  routerPkg --> syncp
  routerPkg --> slog
  middlewarePkg --> nethttp
  middlewarePkg --> slog
```

> **Phase 5 note:** `pkg/middleware` now imports `pkg/errors` (Layer 3 → Layer 1) to
> write the standard JSON error envelope from `Recovery`. `pkg/errors` now imports
> `runtime/debug` for optional stack capture in debug mode. `pkg/middleware` also imports
> `runtime/debug` for the panic goroutine stack. `pkg/di`, `pkg/observability`, `cmd/`,
> and `examples/` remain stubs until their respective phases.

## Layer Rules

- Dependencies point downward only. A package never imports a sibling in its own layer or
  anything above it.
- `pkg/errors` is the foundation: it imports nothing from `pkg/`.
- `pkg/router` does not know that `pkg/middleware` exists; middleware wraps handlers from
  the outside.
- `pkg/observability` is wired in through the middleware chain, not called by the router.
- `pkg/di` is a composition-root tool. Only `cmd/` and `examples/` construct a container.
- Nothing under `pkg/` imports `cmd/` or `examples/`.

## Verifying the Diagram

After each phase, regenerate the real graph and reconcile it with the diagram above:

```bash
source ./environment.sh && go list -deps -f '{{.ImportPath}} -> {{join .Imports " "}}' ./... \
  | grep 'github.com/example/lightweight-http'
```

If the real graph has an edge the diagram does not, either the diagram is stale or the
layering rule was violated. Fix the code first; only update the diagram when the new edge
is legitimate.

## External Dependencies

None. Standard library only (ADR-001). This section must stay empty.
