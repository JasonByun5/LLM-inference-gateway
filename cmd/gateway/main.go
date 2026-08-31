// Command gateway is the inference gateway.
//
// Rung 1: reverse proxy that forwards each request to a single backend.
package main

import (
	"flag"
	"io"
	"llm-inference-gateway/internal/balancer"
	"log"
	"net/http"
	"strings"
	"time"
)

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
	flag.Parse()

	// Outbound: talks to the backend. Timeout so a dead backend → 502 for header response
	client := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 2 * time.Second,
		},
	}

	raw := strings.Split(*backends, ",")
	origins := make([]string, 0, len(raw))
	for _, b := range raw {
		if s := strings.TrimSpace(b); s != "" {
			origins = append(origins, s)
		}
	}
	pool := balancer.New(origins)

	// Inbound: curl hits this. Each request builds a *new* outbound request.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("gateway got %s %s", r.Method, r.URL.RequestURI())

		// picks a backend from the pool and creates the URL
		backend, err := pool.Pick()
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
