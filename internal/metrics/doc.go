// Package metrics will hold latency recording: collect raw duration samples
// and compute p50 / p95 / p99 from them. No estimation tricks -- keep the
// raw samples and sort. Prometheus integration comes later.
//
// Built in rung 4 alongside the load generator.
package metrics
