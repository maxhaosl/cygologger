#!/usr/bin/env bash
#
# cygologger — one-click verification gate.
#
# Runs the whole validation pipeline and exits non-zero if ANY stage fails:
#   1. go build ./...                 (compile everything)
#   2. go vet ./...                  (static analysis)
#   3. go test -race ./...           (functional + stability, race-clean)
#   4. go test -bench (short)        (efficiency benchmarks, -benchmem)
#   5. every example/ (go run .)    (incl. config_verify's 6 -opt sub-processes)
#   6. print a PASS/FAIL feature matrix and return the aggregate status
#
# macOS has no `timeout` command, so stages are not wrapped in timeouts; the
# benchmark step uses a short -benchtime so it never hangs a CI runner.
#
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

PASS=()
FAIL=()

record() {            # $1 = name   $2 = 0 (ok) / 1 (fail)
    if [ "$2" -eq 0 ]; then
        PASS+=("$1")
        printf '  [PASS] %s\n' "$1"
    else
        FAIL+=("$1")
        printf '  [FAIL] %s\n' "$1"
    fi
}

run() {               # $1 = name   rest = command
    local name="$1"; shift
    printf '\n=== [%s] ===\n' "$name"
    if "$@"; then
        record "$name" 0
    else
        record "$name" 1
    fi
}

# --------------------------------------------------------------------------
# 1-4. Core Go pipeline (root module: ICYLogger + sub-packages)
# --------------------------------------------------------------------------
run "build"      go build ./...
run "vet"        go vet ./...
run "test-race"  go test -race ./...
run "bench"      go test -bench=. -benchmem -benchtime 100x -run '^$' ./...

# --------------------------------------------------------------------------
# 5. Examples (each is its own module with a go.mod)
# --------------------------------------------------------------------------
printf '\n=== [examples] ===\n'
for d in examples/*/; do
    [ -f "$d/go.mod" ] || continue
    name="$(basename "$d")"
    if [ "$name" = "config_verify" ]; then
        # config_verify verifies one C++ option per isolated sub-process.
        for opt in console remote sys filemode layout defaults; do
            if ( cd "$d" && go run . -opt="$opt" ) >/dev/null 2>&1; then
                record "example:config_verify($opt)" 0
            else
                record "example:config_verify($opt)" 1
            fi
        done
    elif [ "$name" = "stress_test" ]; then
        # Full 12-test stress suite (correctness + load) with short durations
        # so the gate stays fast; run `go run . -test=all` manually for the
        # long-form soak numbers.
        if ( cd "$d" && go run . -test=all -duration=1s -count=60000 ) >/dev/null 2>&1; then
            record "example:stress_test(12 tests)" 0
        else
            record "example:stress_test(12 tests)" 1
        fi
    else
        if ( cd "$d" && go run . ) >/dev/null 2>&1; then
            record "example:$name" 0
        else
            record "example:$name" 1
        fi
    fi
done

# --------------------------------------------------------------------------
# 6. Summary matrix
# --------------------------------------------------------------------------
printf '\n========================================\n'
printf 'cygologger verification summary\n'
printf '  PASS: %d   FAIL: %d\n' "${#PASS[@]}" "${#FAIL[@]}"
if [ "${#FAIL[@]}" -gt 0 ]; then
    printf -- '----------------------------------------\n'
    printf 'FAILED stages:\n'
    for f in "${FAIL[@]:-}"; do
        [ -n "$f" ] && printf '  - %s\n' "$f"
    done
fi
printf '========================================\n'

[ "${#FAIL[@]}" -eq 0 ]
