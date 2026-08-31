// Command fakellm is a mock LLM inference server used as a test backend.
//
// Rung 1 (temporary): a static "hello from 9001" origin so the gateway has
// something to forward to.
// Rung 2: replace this with fake prefill delay + streamed SSE tokens.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	port := flag.String("port", "9001", "port for the proxy")
	name := flag.String("name", "fakellm", "")
	flag.Parse()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Prompt    string `json:"prompt"`
			MaxTokens int    `json:"max_tokens"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		select {
		case <-r.Context().Done():
			log.Printf("client disconnected")
			return
		case <-time.After(time.Duration(len(body.Prompt)) * time.Millisecond):
		}

		if body.MaxTokens == 0 {
			body.MaxTokens = 50
		}

		words := strings.Fields(body.Prompt)

		if len(words) == 0 {
			words = []string{"token"}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		for i := 0; i < body.MaxTokens; i++ {
			word := words[i%len(words)]
			fmt.Fprintf(w, "data: {\"token\":%q,\"worker\":%q}\n\n", word, *name)
			flusher.Flush()

			select {
			case <-r.Context().Done():
				log.Printf("client disconnected")
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()

	})

	log.Printf("%s listening on :%s", *name, *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
