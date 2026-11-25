#!/usr/bin/env bash
# Acceptance entry: runs go tests covering AC-S1/S3/M1/M2/N1/N4/E1/E2/P1 via stubs + mock Modbus/HTTP.
# Optional external brokers (MQTT/AMQP/OPC UA) are skipped unless ACCEPTANCE_EXTERNAL=1.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
export PATH="${HOME}/go-sdk/go/bin:${PATH:-}"
echo "== go test ./... =="
go test ./... -count=1
echo "== ts cache throughput (>=40000 pts/s) =="
go test ./internal/tscache/ -count=1 -run TestWriteThroughput40kPerSec -v
echo "ACCEPTANCE_CORE=PASS"
if [[ "${ACCEPTANCE_EXTERNAL:-0}" == "1" ]]; then
  echo "External broker checks not automated in this script; use docs/runbook.md + compose."
fi
