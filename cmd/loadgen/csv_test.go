package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llm-inference-gateway/internal/metrics"
)

func TestAppendCSVAddsPolicyColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loadgen.csv")
	old := "label,rate,sent,ok,err_429,err_502,ttft_p50,ttft_p99,total_p50,total_p99\n" +
		"\"3x fakellm, slots=1\",2,60,60,0,0,31.00,47.00,796.00,1070.00\n"
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}

	st := &stats{}
	st.sent.Store(60)
	st.ok.Store(36)
	if err := appendCSV(path, "cache-aware run", "cache-aware", "shared", 3, st, metrics.New(), metrics.New()); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0][1] != "policy" {
		t.Fatalf("header[1] = %q, want policy", rows[0][1])
	}
	if rows[1][1] != "" {
		t.Fatalf("old row policy = %q, want empty", rows[1][1])
	}
	if rows[1][0] != "3x fakellm, slots=1" || rows[1][3] != "2" {
		t.Fatalf("old row shifted: %v", rows[1])
	}
	if rows[2][1] != "cache-aware" {
		t.Fatalf("new row policy = %q", rows[2][1])
	}
	if rows[0][2] != "workload" {
		t.Fatalf("header[2] = %q, want workload", rows[0][2])
	}
	if rows[1][2] != "" {
		t.Fatalf("old row workload = %q, want empty", rows[1][2])
	}
	if rows[2][2] != "shared" {
		t.Fatalf("new row workload = %q", rows[2][2])
	}
}

func TestWorkloads(t *testing.T) {
	sharedA := requestBody("shared", 16)
	sharedB := requestBody("shared", 16)
	if sharedA == sharedB {
		t.Fatal("shared requests should differ in the question number")
	}
	var a, b struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(sharedA), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(sharedB), &b); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.Prompt, sharedPrefix) || !strings.HasPrefix(b.Prompt, sharedPrefix) {
		t.Fatal("shared prompts must start with the same prefix")
	}

	uniqueA := requestBody("unique", 16)
	uniqueB := requestBody("unique", 16)
	var u, v struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(uniqueA), &u); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(uniqueB), &v); err != nil {
		t.Fatal(err)
	}
	if u.Prompt[0] == v.Prompt[0] {
		t.Fatalf("unique prompts share a first byte: %q and %q", u.Prompt, v.Prompt)
	}
}
