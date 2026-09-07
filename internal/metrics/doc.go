// Package metrics records raw latency samples and computes p50 / p95 / p99
// by sorting a copy. Index is int(p * (n-1)). Prometheus integration comes later.
//
// Built in rung 4 alongside the load generator.
package metrics
