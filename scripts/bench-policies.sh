#!/usr/bin/env bash
# Compare routing policies against three llama-servers already up on
# :9001, :9002, and :9003.
#
# Conditions
#   control        one server (9001), so there is nothing to route
#   round-robin    all three servers
#   least-inflight all three servers
#   cache-aware    all three servers
#
# Workloads
#   shared   same long prefix, different question at the end.
#            This is the case where cache-aware routing can help.
#   unique   no common opening bytes.
#            Cache-aware should look like least-inflight here.
#
# Each workload x rate x policy runs REPS times. Policy order rotates
# every rep, and every llama slot is erased between runs, so one
# policy does not inherit a warm KV cache from the previous one.
#
# SLOTS is how many sequences each worker may run at once. It must match
# llama-server --parallel. One slot holds one prompt; a later different
# prompt on that same slot replaces it. Extra slots let the glossary stay
# open in one slot while another prompt uses a different slot.
#
# Rates are requests/second. 0.5 leaves a gap between requests, which is
# where a prefix hit shows up in time to first token. 1 and 2 overlap,
# which is where pinning every request to one worker starts to hurt.
#
# Cache-aware weights: a full prefix is about 4 blocks of 256 bytes.
# score = 6*matched - 20*inflight, so a second in-flight request on the
# cached worker loses to an idle worker. The prefix is reused when the
# worker is free, and copied to another worker when it is busy.
#
# Override with env vars: RATES, REPS, DURATION, WORKLOADS, CSV, SLOTS.
set -euo pipefail
cd "$(dirname "$0")/.."

RATES=(${RATES:-0.5 1 2})
REPS=${REPS:-3}
DURATION=${DURATION:-30s}
WORKLOADS=(${WORKLOADS:-shared unique})
CSV=${CSV:-loadgen.csv}
SLOTS=${SLOTS:-4}
ALL="http://127.0.0.1:9001,http://127.0.0.1:9002,http://127.0.0.1:9003"
ONE="http://127.0.0.1:9001"
ROOT="/tmp/llm-gateway-bench"
LOGS="$ROOT/logs"
mkdir -p "$LOGS" bin

go build -o bin/gateway ./cmd/gateway
go build -o bin/loadgen ./cmd/loadgen

for port in 9001 9002 9003; do
	if ! curl -sf -o /dev/null "http://127.0.0.1:${port}/health"; then
		echo "llama-server is not healthy on :${port}" >&2
		exit 1
	fi
done

if (echo >/dev/tcp/127.0.0.1/8080) >/dev/null 2>&1; then
	echo "port 8080 is already in use. Stop the gateway already running there, then run this again." >&2
	exit 1
fi

gw_pid=""
cleanup() {
	if [[ -n "${gw_pid}" ]] && kill -0 "$gw_pid" 2>/dev/null; then
		kill "$gw_pid" 2>/dev/null || true
		wait "$gw_pid" 2>/dev/null || true
	fi
	gw_pid=""
}
trap cleanup EXIT

flush_slots() {
	local port id ids
	for port in 9001 9002 9003; do
		ids=$(curl -sf "http://127.0.0.1:${port}/slots" | python3 -c 'import json,sys; print(" ".join(str(s["id"]) for s in json.load(sys.stdin)))') \
			|| { echo "warning: could not list slots on :${port}" >&2; continue; }
		for id in $ids; do
			curl -sf -o /dev/null -X POST "http://127.0.0.1:${port}/slots/${id}?action=erase" \
				|| echo "warning: erase of :${port} slot ${id} failed" >&2
		done
	done
}

start_gateway() {
	local policy=$1
	local backends=$2
	local log=$3
	cleanup
	local args=(-backends "$backends" -policy "$policy" -slots "$SLOTS" -llama)
	if [[ "$policy" == "cache-aware" ]]; then
		args+=(-cache-block-size 256 -overlap-weight 6 -load-penalty 20)
	fi
	./bin/gateway "${args[@]}" >"$log" 2>&1 &
	gw_pid=$!
	local i
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
		if (echo >/dev/tcp/127.0.0.1/8080) >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.2
	done
	echo "gateway failed to listen. log: $log" >&2
	tail -n 30 "$log" >&2 || true
	exit 1
}

policies=(control round-robin least-inflight cache-aware)
npolicies=${#policies[@]}

for workload in "${WORKLOADS[@]}"; do
	for rate in "${RATES[@]}"; do
		for ((rep = 1; rep <= REPS; rep++)); do
			for ((k = 0; k < npolicies; k++)); do
				policy=${policies[$(((k + rep - 1) % npolicies))]}
				backends="$ALL"
				gw_policy="$policy"
				if [[ "$policy" == "control" ]]; then
					backends="$ONE"
					gw_policy="round-robin"
				fi
				echo "---- workload=${workload} rate=${rate} policy=${policy} rep=${rep}"
				flush_slots
				start_gateway "$gw_policy" "$backends" "$LOGS/${workload}-${policy}-r${rate}-rep${rep}.log"
				./bin/loadgen \
					-rate "$rate" \
					-duration "$DURATION" \
					-workload "$workload" \
					-policy "$policy" \
					-label "rep${rep}" \
					-max-tokens 16 \
					-csv "$CSV"
			done
		done
	done
done

cleanup
trap - EXIT

python3 - "$CSV" <<'PY'
import csv, sys
from collections import defaultdict
path = sys.argv[1]
groups = defaultdict(list)
with open(path, newline="") as f:
    for row in csv.DictReader(f):
        if not row.get("workload"):
            continue
        groups[(row["workload"], row["policy"], row["rate"])].append(row)

print("\nmean of the per-run medians (ttft_p50 / total_p50, milliseconds)")
print(f"{'workload':8} {'policy':16} {'rate':6} {'reps':4} {'ttft_p50':10} {'total_p50':10} {'ok':8}")
for key in sorted(groups, key=lambda k: (k[0], float(k[2]), k[1])):
    rows = groups[key]
    def mean(col):
        return sum(float(r[col]) for r in rows) / len(rows)
    ok = sum(float(r["ok"]) for r in rows) / len(rows)
    print(f"{key[0]:8} {key[1]:16} {key[2]:6} {len(rows):4} {mean('ttft_p50'):10.1f} {mean('total_p50'):10.1f} {ok:8.1f}")
PY
