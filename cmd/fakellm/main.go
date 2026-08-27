// Command fakellm is a mock LLM inference server used as a test backend.
//
// Rung 1 (temporary): a static "hello from 9001" origin so the gateway has
// something to forward to.
// Rung 2: replace this with fake prefill delay + streamed SSE tokens.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

func main() {
	port := flag.String("port", "http:localhost:9001", "port for the proxy")
	name := flag.String("name", "fakellm", "")
	flag.Parse()

	http.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Prompt    string `"json:prompt"`
			MaxTokens int    `"json:max_tokens"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		if body.MaxTokens == 0 {
			body.MaxTokens = 50
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"prompt":     body.Prompt,
			"max_tokens": body.MaxTokens,
		})

	})

	log.Printf("%s listening on :%s", *name, *port)
	log.Fatal(http.ListenAndServe(":9001", nil))
}
