package balancer

import "testing"

func TestPickRoundRobin(t *testing.T) {
	p := New([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	})
	want := []string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
		"http://localhost:9001",
	}
	for i, u := range want {
		b, err := p.Pick()
		if err != nil {
			t.Fatal(err)
		}
		if b.URL != u {
			t.Fatalf("pick %d: got %s want %s", i, b.URL, u)
		}
	}
}

func TestPickSkipsUnhealthy(t *testing.T) {
	p := New([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	})
	p.backends[1].healthy = false // 9002 down

	want := []string{
		"http://localhost:9001",
		"http://localhost:9003",
		"http://localhost:9001",
	}
	for i, u := range want {
		b, err := p.Pick()
		if err != nil {
			t.Fatal(err)
		}
		if b.URL != u {
			t.Fatalf("pick %d: got %s want %s", i, b.URL, u)
		}
	}
}
