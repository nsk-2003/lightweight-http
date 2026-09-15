// Purpose: Package observability provides request tracing, in-process metrics, and structured
// logging, each exposed as composable middleware (ADR-011, ADR-012).
//
// # Recommended Middleware Order
//
// Register middleware on the router in this order so that each layer sees the full context
// established by the layers above it:
//
//	ro.Use(
//	    observability.RequestID(),          // 1 — outermost: assigns request ID, echoes header
//	    observability.LoggingMiddleware(l), // 2 — logs after inner layers complete
//	    observability.MetricsMiddleware(c), // 3 — records latency/status after recovery runs
//	    middleware.Recovery(l),             // 4 — converts panics into 500 responses
//	)
//	// handler registered last is the innermost target
//
// Placing Recovery inside MetricsMiddleware ensures that panicking handlers still produce
// a 500 status code that is recorded by the metrics layer.
package observability
