// Command loadgen fires requests at the gateway and reports latency percentiles.
//
// Rung 4: implement a load tester that
//   - sends requests at a configurable rate (open-loop: don't wait for
//     responses before sending the next request),
//   - records per-request latency,
//   - prints p50 / p95 / p99 at the end.
package main

import (
	"fmt"
	"os"
)

func main() {
	// TODO(rung 4): implement the open-loop load generator.
	fmt.Fprintln(os.Stderr, "loadgen: not implemented yet (rung 4)")
	os.Exit(1)
}
