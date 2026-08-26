// Package proxy will hold the request-forwarding logic shared by the gateway:
// forwarding a request to a backend, and (later) relaying a streaming
// response chunk-by-chunk with client-disconnect cancellation.
//
// Built in rungs 1-2.
package proxy
