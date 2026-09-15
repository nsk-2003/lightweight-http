# Package Design

This document describes the intended module layout and dependencies for the lightweight
HTTP framework. It is the **target** design; update it after each phase so it matches the
real import graph.

## Target Module Dependency Diagram

The diagram shows the full target architecture. Edges in **bold** are implemented; remaining
edges are planned for later phases. Updated after each phase to match the real import graph.

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
  routerPkg -.->|planned Phase 5| errorsPkg
  diPkg --> errorsPkg
```

### Phase 2 — Real Import Graph (pkg/router)

`pkg/router` is implemented and imports only the Go standard library:
`context`, `fmt`, `net/http`, `strings`, `sync`. No intra-module edges yet.

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
