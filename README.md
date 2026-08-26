# LLM Inference Gateway

A cache-aware load balancer for LLM serving, built incrementally from first
principles. The gateway sits in front of multiple inference workers
(llama.cpp) and handles streaming, load balancing, admission control, and
prefix-cache-aware routing.

## Status

Work in progress — built as a ladder of self-contained rungs, each becoming a
component of the final system:

- [ ] **Rung 1 — Reverse proxy** (`cmd/gateway`): forward requests to a single backend
- [ ] **Rung 2 — Fake LLM server** (`cmd/fakellm`): mock inference server with fake
      prefill delay and streamed "tokens" (SSE)
- [ ] **Rung 3 — Load balancer** (`internal/balancer`): multiple backends,
      health checks, round-robin and least-inflight policies
- [ ] **Rung 4 — Load generator** (`cmd/loadgen`, `internal/metrics`): open-loop
      load testing with p50/p95/p99 reporting
- [ ] **Rung 5 — Data structures** (`internal/lru`, `internal/radix`): LRU cache
      and radix tree, test-first
- [ ] **Integration**: swap fake workers for llama.cpp, add queueing/shedding,
      cache-aware routing, benchmarks

## Layout

```
cmd/gateway/    the gateway binary (grows from proxy to full router)
cmd/fakellm/    mock LLM worker used as a test backend
cmd/loadgen/    load tester
internal/       shared packages (proxy, balancer, lru, radix, metrics)
```

## Running

```sh
go build ./...
go test ./...
```
