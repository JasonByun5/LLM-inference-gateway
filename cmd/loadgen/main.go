// Command loadgen fires requests at the gateway and reports latency percentiles.
//
// Rung 4: implement a load tester that
//   - sends requests at a configurable rate (open-loop: don't wait for
//     responses before sending the next request),
//   - records per-request latency,
//   - prints p50 / p95 / p99 at the end.
package main

import (
	"flag"
	"fmt"
	"io"
	"llm-inference-gateway/internal/metrics"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var requests = []string{
	`{"prompt":"This is prompt 1 from loadgen","max_tokens":15}`,
	`{"prompt":"This is prompt 2 from loadgen and is longer", "max_tokens":20}`,
	`{"prompt":"Prompt 3 from loadgen","max_tokens":10}`,
	`{"prompt":"This is prompt 4 from loadgen","max_tokens":15}`,
}

func sendOne(url string, request string, ttft *metrics.Recorder, total *metrics.Recorder, errors *atomic.Int64, wg *sync.WaitGroup) {
	defer wg.Done()

	body := strings.NewReader(request)
	t0 := time.Now()

	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		errors.Add(1)
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errors.Add(1)
		io.Copy(io.Discard, resp.Body)
		return
	}

	buf := make([]byte, 4096)
	gotFirst := false
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 && !gotFirst {
			ttft.Record(time.Since(t0))
			gotFirst = true
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			errors.Add(1)
			return
		}
	}

	total.Record(time.Since(t0))
}

func main() {
	rate := flag.Int("rate", 1, "# of requests per second")
	duration := flag.Duration("duration", 20*time.Second, "how long (in sec) requests will be sent")
	url := flag.String("url", "http://localhost:8080/generate", "where the requests will be sent")
	flag.Parse()

	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()

	stop := time.After(*duration)
	var wg sync.WaitGroup
	ttft := metrics.New()
	total := metrics.New()
	var sent, errors atomic.Int64

	for {
		select {
		case <-stop:
			ticker.Stop()
			wg.Wait()
			fmt.Printf("sent=%d   errors=%d\n", sent.Load(), errors.Load())
			fmt.Printf("ttft:  p50=%v p95=%v p99=%v\n",
				ttft.Percentile(0.50), ttft.Percentile(0.95), ttft.Percentile(0.99))
			fmt.Printf("total:  p50=%v p95=%v p99=%v\n",
				total.Percentile(0.50), total.Percentile(0.95), total.Percentile(0.99))

			return
		case <-ticker.C:
			sent.Add(1)
			wg.Add(1)
			request := requests[rand.Intn(len(requests))]
			go sendOne(*url, request, ttft, total, &errors, &wg)

		}
	}
}
