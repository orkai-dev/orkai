#!/usr/bin/env bash
set -euo pipefail

profile="${1:-coverage.out}"
if [[ ! -f "$profile" ]]; then
  echo "coverage profile not found: $profile" >&2
  exit 1
fi

awk -v k3s_floor=5 -v jobs_floor=60 -v ws_floor=25 '
/^mode:/ { next }
NF < 3 { next }
{
  n = split($0, a, " ")
  stmts = a[n-1] + 0
  count = a[n] + 0
  covered_add = (count > 0) ? stmts : 0
  path = $0
  sub(/:[0-9].*$/, "", path)
  if (path ~ /\/internal\/orchestrator\/k3s\//) {
    kt += stmts; kc += covered_add
  } else if (path ~ /\/internal\/jobs\//) {
    jt += stmts; jc += covered_add
  } else if (path ~ /\/internal\/api\/ws\//) {
    wt += stmts; wc += covered_add
  }
}
function pct(c, t) { return t == 0 ? 0 : int(c * 100 / t) }
function check(name, c, t, floor) {
  p = pct(c, t)
  printf "%s: %d%% (%d/%d stmts, floor %d%%)\n", name, p, c, t, floor
  if (t == 0) {
    printf "FAIL %s: no statements in profile (floor %d%%)\n", name, floor > "/dev/stderr"
    failed = 1
  } else if (p < floor) {
    printf "FAIL %s below floor (%d%% < %d%%)\n", name, p, floor > "/dev/stderr"
    failed = 1
  }
}
END {
  check("internal/orchestrator/k3s", kc, kt, k3s_floor)
  check("internal/jobs", jc, jt, jobs_floor)
  check("internal/api/ws", wc, wt, ws_floor)
  exit failed
}
' "$profile"
