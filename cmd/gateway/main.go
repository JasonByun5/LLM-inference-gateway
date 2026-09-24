// Command gateway is the inference gateway.
//
// Rung 1: reverse proxy that forwards each request to a single backend.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"llm-inference-gateway/internal/balancer"
	"llm-inference-gateway/internal/queue"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type generateReq struct {
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"max_tokens"`
}

// rewriteGenerate maps the loadgen/client POST /generate body onto
// llama-server's /v1/chat/completions API
func rewriteGenerate(r *http.Request) error {
	if r.URL.Path != "/generate" {
		return nil
	}

	var in generateReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		r.Body.Close()
		return err
	}
	r.Body.Close()

	if in.MaxTokens == 0 {
		in.MaxTokens = 50
	}

	out, err := json.Marshal(map[string]any{
		"messages":   []map[string]string{{"role": "user", "content": in.Prompt}},
		"max_tokens": in.MaxTokens,
		"stream":     true,
	})
	if err != nil {
		return err
	}

	r.Body = io.NopCloser(bytes.NewReader(out))
	r.ContentLength = int64(len(out))
	r.URL.Path = "/v1/chat/completions"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", strconv.Itoa(len(out)))
	return nil
}

// promptFrom reads a /generate JSON body and returns its prompt.
// The body is put back so rewriteGenerate or the proxy can read it again.
func promptFrom(r *http.Request) (string, error) {
	if r.URL.Path != "/generate" || r.Body == nil {
		return "", nil
	}
	raw, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	r.ContentLength = int64(len(raw))
	r.Header.Set("Content-Length", strconv.Itoa(len(raw)))

	var in generateReq
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	return in.Prompt, nil
}

type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	fw.f.Flush()
	return n, err
}

var hopByHop = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func copySkipHop(dst, src http.Header) {
	for k, vs := range src {
		if hopByHop[k] {
			continue
		}
		dst[k] = append([]string(nil), vs...)
	}

	for _, f := range strings.Split(src.Get("Connection"), ",") {
		if f = strings.TrimSpace(f); f != "" {
			dst.Del(http.CanonicalHeaderKey(f))
		}
	}
}

func main() {
	backends := flag.String("backends", "http://localhost:9001", "origin to forward to")
	policyName := flag.String("policy", "round-robin", "round-robin or least-inflight or cache-aware")
	llama := flag.Bool("llama", false, "rewrite POST /generate to llama-server /v1/chat/completions")
	queueSize := flag.Int("queue-size", 8, "max requests waiting for a slot")
	maxWait := flag.Duration("max-wait", 2*time.Second, "shed when estimated wait exceeds this")
	slots := flag.Int("slots", 1, "concurrent requests per backend")

	cacheBlockSize := flag.Int("cache-block-size", 64, "prompt bytes per cache block")
	overlapWeight := flag.Float64("overlap-weight", 10, "score added per matched block")
	loadPenalty := flag.Float64("load-penalty", 1, "score subtracted per inflight request")
	cacheBlocks := flag.Int("cache-blocks", 256, "predicted cache capacity in blocks per worker")
	cacheTTL := flag.Duration("cache-ttl", 30*time.Second, "how long a predicted cache entry stays valid")
	flag.Parse()

	var policy balancer.Policy
	switch *policyName {
	case "least-inflight":
		policy = balancer.LeastInflight
	case "cache-aware":
		policy = balancer.CacheAware
	case "round-robin":
		policy = balancer.RoundRobin
	default:
		log.Fatalf("unknown policy %q", *policyName)
	}

	// Outbound: talks to the backend. Timeout so a dead backend → 502 for header response
	client := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	raw := strings.Split(*backends, ",")
	origins := make([]string, 0, len(raw))
	for _, b := range raw {
		if s := strings.TrimSpace(b); s != "" {
			origins = append(origins, s)
		}
	}
	pool := balancer.New(origins, policy, balancer.CacheConfig{
		OverlapWeight: *overlapWeight,
		LoadPenalty:   *loadPenalty,
		Capacity:      *cacheBlocks,
		TTL:           *cacheTTL,
	})
	q := queue.New(*queueSize, *maxWait, func() int {
		return pool.Healthy() * *slots
	})

	go pool.CheckHealth()

	// Inbound: curl hits this. Each request builds a *new* outbound request.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("gateway got %s %s", r.Method, r.URL.RequestURI())

		// Blocks are what Pick scores against. Other policies ignore them.
		// Read the prompt before rewriteGenerate replaces the body.
		var blocks []uint64
		if policy == balancer.CacheAware {
			prompt, err := promptFrom(r)
			if err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			blocks = splitBlocks(prompt, *cacheBlockSize)
		}

		// Must run before URL + body are forwarded: mutates path, body, Content-Length.
		if *llama {
			if err := rewriteGenerate(r); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
		}

		//checks the queue to make sure that it is within bounds
		if err := q.Acquire(r.Context()); err != nil {
			switch err {
			case queue.ErrShed:
				http.Error(w, "too many requests", http.StatusTooManyRequests)
			case queue.ErrNoCapacity:
				http.Error(w, "bad gateway", http.StatusBadGateway)
			}
			return
		}

		start := time.Now()
		defer func() { q.Release(time.Since(start)) }()

		// picks a backend from the pool and creates the URL
		backend, err := pool.Pick(blocks)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		defer pool.Release(backend)
		url := backend.URL + r.URL.RequestURI()

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// r.Body is a stream — pass it through; do not ReadAll then try again.
		outReq, err := http.NewRequestWithContext(r.Context(), r.Method, url, r.Body)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		copySkipHop(outReq.Header, r.Header)
		outReq.ContentLength = r.ContentLength

		resp, err := client.Do(outReq)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copySkipHop(w.Header(), resp.Header)

		w.WriteHeader(resp.StatusCode)
		io.Copy(flushWriter{w, flusher}, resp.Body)
	})

	log.Printf("gateway listening on :8080 (backend=%v)", origins)
	log.Fatal(http.ListenAndServe(":8080", handler))
}
