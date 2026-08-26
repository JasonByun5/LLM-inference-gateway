// Command fakellm is a mock LLM inference server used as a test backend.
//
// Rung 1 (temporary): a static "hello from 9001" origin so the gateway has
// something to forward to.
// Rung 2: replace this with fake prefill delay + streamed SSE tokens.
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("fakellm got %s %s", r.Method, r.URL.RequestURI())
		fmt.Fprint(w, "hello from 9001")
	})
	log.Println("fakellm listening on :9001")
	log.Fatal(http.ListenAndServe(":9001", nil))
}
