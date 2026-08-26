// Command gateway is the inference gateway.
//
// Rung 1: reverse proxy that forwards each request to a single backend.
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	backend := flag.String("backend", "http://localhost:9001", "origin to forward to")
	flag.Parse()

	// Outbound: talks to the backend. Timeout so a dead backend → 502, not a hang.
	client := &http.Client{Timeout: 2 * time.Second}

	// Inbound: curl hits this. Each request builds a *new* outbound request.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("gateway got %s %s", r.Method, r.URL.RequestURI())

		// Same path and query the client asked for, on the backend host.
		url := *backend + r.URL.RequestURI()

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
		io.Copy(w, resp.Body)
	})

	log.Printf("gateway listening on :8080 (backend=%s)", *backend)
	log.Fatal(http.ListenAndServe(":8080", handler))
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
