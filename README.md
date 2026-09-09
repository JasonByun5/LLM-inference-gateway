# LLM Inference Gateway

A cache-aware load balancer for LLM serving, built incrementally from first
principles. The gateway sits in front of multiple inference workers
(llama.cpp) and handles streaming, load balancing, admission control, and
prefix-cache-aware routing.

## Status

Work in progress — built as a ladder of self-contained rungs, each becoming a
component of the final system:

- [x] **Rung 1 — Reverse proxy** (`cmd/gateway`): forward requests to a single backend
- [X] **Rung 2 — Fake LLM server** (`cmd/fakellm`): mock inference server with fake
  ```
  prefill delay and streamed "tokens" (SSE)
  ```
- [X] **Rung 3 — Load balancer** (`internal/balancer`): multiple backends,
  ```
  health checks, round-robin and least-inflight policies
  ```
- [ ] **Rung 4 — Load generator** (`cmd/loadgen`, `internal/metrics`): open-loop
  ```
  load testing with p50/p95/p99 reporting
  ```
- [ ] **Rung 5 — Data structures** (`internal/lru`, `internal/radix`): LRU cache
  ```
  and radix tree, test-first
  ```
- [ ] **Integration**: swap fake workers for llama.cpp, add queueing/shedding,
  ```
  cache-aware routing, benchmarks
  ```

## Rung 4 — load sweep
Setup: 3 fakellm workers (`-slots 1`), gateway on `:8080`, 30s open-loop.
Capacity ≈ 3 / 0.8s ≈ 4 req/s.
| rate | sent | errors | ttft p50 | ttft p99 | total p50 | total p99 |
|------|------|--------|----------|----------|-----------|-----------|
| 2    | 60   | 0      | 31ms     | 47ms     | 796ms     | 1.07s     |
| 3    | 90   | 0      | 31ms     | 170ms    | 797ms     | 1.13s     |
| 5    | 150  | 30     | 1.61s    | 1.98s    | 2.34s     | 2.97s     |
| 8    | 240  | 118    | 1.80s    | 1.99s    | 2.54s     | 3.01s     |
| 10   | 300  | 179    | 1.86s    | 1.99s    | 2.59s     | 2.96s     |
Under ~4 req/s, total stays ~800ms. At 5 req/s, p99 jumps (queue).
Higher rates add 502s from the gateway header timeout, so successful
p99 sits near ~3s instead of climbing.


## Layout

```
cmd/gateway/    the gateway binary (cgrows from proxy to full router)
cmd/fakellm/    mock LLM worker used as a test backend
cmd/loadgen/    load tester
internal/       shared packages (proxy, balancer, lru, radix, metrics)
```



## Running

```sh
go build ./...
go test ./...
```

