// Command loadgen fires requests at the gateway and reports latency percentiles.
//
// Rung 4: implement a load tester that
//   - sends requests at a configurable rate (open-loop: don't wait for
//     responses before sending the next request),
//   - records per-request latency,
//   - prints p50 / p95 / p99 at the end.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"llm-inference-gateway/internal/metrics"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// sharedPrefix is long enough that prefill dominates, and identical on every
// shared-workload request. Only the question number at the end changes.
const sharedPrefix = "You are a careful assistant for the inference gateway benchmark. " +
	"Use the glossary exactly. Alpha means the first worker. Beta means the second worker. " +
	"Gamma means the third worker. A slot is one sequence in the KV cache. " +
	"A prefix hit skips prefill for the bytes already cached on that worker. " +
	"Round robin spreads requests with no regard for that cache. " +
	"Least inflight spreads requests toward the worker with the fewest active requests. " +
	"Cache aware routing sends a prompt to the worker predicted to hold the longest matching prefix, " +
	"unless that worker is busy enough that the load penalty wins. " +
	"The glossary continues. Delta means queue wait. Epsilon means time to first token. " +
	"Zeta means total latency. Eta means a shed request. Theta means a healthy backend. " +
	"Repeat the definitions when asked, and answer the question that follows them. "

var promptSeq atomic.Int64

func marshalPrompt(prompt string, maxTokens int) string {
	b, err := json.Marshal(map[string]any{
		"prompt":     prompt,
		"max_tokens": maxTokens,
	})
	if err != nil {
		return `{"prompt":"fallback","max_tokens":16}`
	}
	return string(b)
}

func requestBody(workload string, maxTokens int) string {
	switch workload {
	case "shared":
		n := promptSeq.Add(1)
		return marshalPrompt(sharedPrefix+fmt.Sprintf(" Question %d.", n), maxTokens)
	case "unique":
		n := promptSeq.Add(1)
		return marshalPrompt(fmt.Sprintf("%d is a standalone request about subject %d and salt %d.", n, n, n), maxTokens)
	default:
		return requests[rand.Intn(len(requests))]
	}
}

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

func csvHeader() []string {
	return []string{
		"label", "policy", "workload", "rate", "sent", "ok", "err_429", "err_502",
		"ttft_p50", "ttft_p99", "total_p50", "total_p99",
	}
}

func hasColumn(header []string, name string) bool {
	for _, c := range header {
		if c == name {
			return true
		}
	}
	return false
}

func insertAt(row []string, i int, v string) []string {
	if i > len(row) {
		i = len(row)
	}
	out := make([]string, 0, len(row)+1)
	out = append(out, row[:i]...)
	out = append(out, v)
	out = append(out, row[i:]...)
	return out
}

func ensureColumn(rows [][]string, name string, at int) [][]string {
	if hasColumn(rows[0], name) {
		return rows
	}
	rows[0] = insertAt(rows[0], at, name)
	for i := 1; i < len(rows); i++ {
		rows[i] = insertAt(rows[i], at, "")
	}
	return rows
}

func appendCSV(path string, label string, policy string, workload string, rate float64, st *stats, ttft, total *metrics.Recorder) error {
	var rows [][]string
	if b, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(b)) > 0 {
		recs, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
		if err != nil {
			return err
		}
		rows = recs
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if len(rows) == 0 {
		rows = [][]string{csvHeader()}
	} else {
		rows = ensureColumn(rows, "policy", 1)
		rows = ensureColumn(rows, "workload", 2)
	}

	rows = append(rows, []string{
		label,
		policy,
		workload,
		strconv.FormatFloat(rate, 'f', -1, 64),
		strconv.FormatInt(st.sent.Load(), 10),
		strconv.FormatInt(st.ok.Load(), 10),
		strconv.FormatInt(st.err429.Load(), 10),
		strconv.FormatInt(st.err502.Load(), 10),
		ms(ttft.Percentile(0.50)),
		ms(ttft.Percentile(0.99)),
		ms(total.Percentile(0.50)),
		ms(total.Percentile(0.99)),
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	return w.Error()
}

func main() {
	rate := flag.Float64("rate", 1, "requests per second")
	duration := flag.Duration("duration", 20*time.Second, "how long (in sec) requests will be sent")
	url := flag.String("url", "http://localhost:8080/generate", "where the requests will be sent")
	csvPath := flag.String("csv", "loadgen.csv", "append a results row to this CSV file (empty to skip)")
	label := flag.String("label", "", "what this run is (e.g. \"3x gemma-4-e2b-it-q4, slots=1\")")
	policy := flag.String("policy", "", "routing policy this run measured (round-robin, least-inflight, cache-aware)")
	workload := flag.String("workload", "mixed", "mixed (old prompts), shared (common prefix), or unique (no common prefix)")
	maxTokens := flag.Int("max-tokens", 16, "generation length for shared and unique workloads")
	flag.Parse()

	switch *workload {
	case "mixed", "shared", "unique":
	default:
		log.Fatalf("unknown workload %q", *workload)
	}

	if *rate <= 0 {
		log.Fatalf("rate must be positive, got %v", *rate)
	}
	ticker := time.NewTicker(time.Duration(float64(time.Second) / *rate))
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
			if *policy != "" {
				fmt.Printf("policy=%s\n", *policy)
			}
			fmt.Printf("workload=%s\n", *workload)
			fmt.Printf("sent=%d  ok=%d  err_429=%d  err_502=%d\n",
				st.sent.Load(), st.ok.Load(), st.err429.Load(), st.err502.Load())
			fmt.Printf("ttft:  p50=%v p95=%v p99=%v\n",
				ttft.Percentile(0.50), ttft.Percentile(0.95), ttft.Percentile(0.99))
			fmt.Printf("total:  p50=%v p95=%v p99=%v\n",
				total.Percentile(0.50), total.Percentile(0.95), total.Percentile(0.99))

			if *csvPath != "" {
				if err := appendCSV(*csvPath, *label, *policy, *workload, *rate, &st, ttft, total); err != nil {
					fmt.Fprintf(os.Stderr, "csv: %v\n", err)
					os.Exit(1)
				}
			}
			return
		case <-ticker.C:
			st.sent.Add(1)
			wg.Add(1)
			request := requestBody(*workload, *maxTokens)
			go sendOne(*url, request, ttft, total, &st, &wg)
		}
	}
}
