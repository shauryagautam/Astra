# Astra Benchmarks

This directory contains the official benchmark suites for the Astra framework. We believe in providing honest, reproducible performance data to help you make informed decisions.

## How to Run Benchmarks

You can run all benchmarks using the standard Go toolchain:

```bash
# Run all benchmarks from the root
go test -v -bench=. ./benchmark/...

# Run specific suite (e.g., Routing)
go test -v -bench=BenchmarkRouter ./benchmark/...
```

## Performance at a Glance

Typical results on a mid-range development machine (measured in Go 1.22+):

| Suite | Operation | Time (ns/op) | Aligned Bytes | Allocs/op |
| --- | --- | --- | --- | --- |
| **Routing** | Static Route | ~30 ns | 0 B | 0 |
| **Routing** | Parametrized | ~150 ns | 16 B | 1 |
| **JSON** | Small Object | ~200 ns | 128 B | 2 |
| **JSON** | Large Array | ~1,200 ns | 4 KB | 3 |

## Interpreting Results

- **ns/op**: Nanoseconds per operation. Lower is better.
- **B/op**: Bytes allocated per operation. Lower is better.
- **allocs/op**: Number of distinct memory allocations per operation. Lower is better.

## Guardrails & Methodology

1. **Reality-based**: We benchmark typical use cases (e.g., routing with parameters and middleware), not just empty function calls.
2. **Standard Hardware**: Unless otherwise noted, official results are generated on standardized cloud instances to ensure comparability.
3. **No Cheating**: We do not use unsafe or misleading micro-optimizations that wouldn't be present in a real application.

---

## Current Suites

- **[Routing](routing_test.go)**: Measures URL matching, parameter extraction, and middleware overhead.
- **[JSON](json_test.go)**: Measures serialization/deserialization performance using the Bytedance Sonic engine.
- **ORM**: (Coming Soon) Measures model scanning and relationship assembly.
