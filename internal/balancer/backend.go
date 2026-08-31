package balancer

type Backend struct {
	URL       string
	inflight  int
	healthy   bool
	fails     int
	successes int
}
