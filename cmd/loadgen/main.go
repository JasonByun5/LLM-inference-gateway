// Command loadgen fires requests at the gateway and reports latency percentiles.
//
// Rung 4: implement a load tester that
//   - sends requests at a configurable rate (open-loop: don't wait for
//     responses before sending the next request),
//   - records per-request latency,
//   - prints p50 / p95 / p99 at the end.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"llm-inference-gateway/internal/metrics"
	"math/rand"
	"net/http"
	"os"
	"strconv"
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

type stats struct {
	sent   atomic.Int64
	ok     atomic.Int64
	err429 atomic.Int64
	err502 atomic.Int64
}

func sendOne(url string, request string, ttft *metrics.Recorder, total *metrics.Recorder, st *stats, wg *sync.WaitGroup) {
	defer wg.Done()

	body := strings.NewReader(request)
	t0 := time.Now()

	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			st.err429.Add(1)
		case http.StatusBadGateway:
			st.err502.Add(1)
		}
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
			return
		}
	}

	total.Record(time.Since(t0))
	st.ok.Add(1)
}

func ms(d time.Duration) string {
	return strconv.FormatFloat(float64(d.Microseconds())/1000.0, 'f', 2, 64)
}

func appendCSV(path string, label string, rate int, st *stats, ttft, total *metrics.Recorder) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	if info.Size() == 0 {
		if err := w.Write([]string{
			"label", "rate", "sent", "ok", "err_429", "err_502",
			"ttft_p50", "ttft_p99", "total_p50", "total_p99",
		}); err != nil {
			return err
		}
	}

	if err := w.Write([]string{
		label,
		strconv.Itoa(rate),
		strconv.FormatInt(st.sent.Load(), 10),
		strconv.FormatInt(st.ok.Load(), 10),
		strconv.FormatInt(st.err429.Load(), 10),
		strconv.FormatInt(st.err502.Load(), 10),
		ms(ttft.Percentile(0.50)),
		ms(ttft.Percentile(0.99)),
		ms(total.Percentile(0.50)),
		ms(total.Percentile(0.99)),
	}); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func main() {
	rate := flag.Int("rate", 1, "# of requests per second")
	duration := flag.Duration("duration", 20*time.Second, "how long (in sec) requests will be sent")
	url := flag.String("url", "http://localhost:8080/generate", "where the requests will be sent")
	csvPath := flag.String("csv", "loadgen.csv", "append a results row to this CSV file (empty to skip)")
	label := flag.String("label", "", "what this run is (e.g. \"3x gemma-4-e2b-it-q4, slots=1\")")
	flag.Parse()

	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()

	stop := time.After(*duration)
	var wg sync.WaitGroup
	ttft := metrics.New()
	total := metrics.New()
	var st stats

	for {
		select {
		case <-stop:
			ticker.Stop()
			wg.Wait()
			if *label != "" {
				fmt.Printf("label=%s\n", *label)
			}
			fmt.Printf("sent=%d  ok=%d  err_429=%d  err_502=%d\n",
				st.sent.Load(), st.ok.Load(), st.err429.Load(), st.err502.Load())
			fmt.Printf("ttft:  p50=%v p95=%v p99=%v\n",
				ttft.Percentile(0.50), ttft.Percentile(0.95), ttft.Percentile(0.99))
			fmt.Printf("total:  p50=%v p95=%v p99=%v\n",
				total.Percentile(0.50), total.Percentile(0.95), total.Percentile(0.99))

			if *csvPath != "" {
				if err := appendCSV(*csvPath, *label, *rate, &st, ttft, total); err != nil {
					fmt.Fprintf(os.Stderr, "csv: %v\n", err)
					os.Exit(1)
				}
			}
			return
		case <-ticker.C:
			st.sent.Add(1)
			wg.Add(1)
			request := requests[rand.Intn(len(requests))]
			go sendOne(*url, request, ttft, total, &st, &wg)
		}
	}
}
