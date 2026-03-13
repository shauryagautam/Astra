# Astra Benchmarks

This directory contains the official benchmark suites for the Astra framework. We believe in providing honest, reproducible performance data to help you make informed decisions.

## How to Run Benchmarks

You can run all benchmarks using the standard Go toolchain:

```bash
# Run all benchmarks
go test -bench=. ./benchmarks/...

# Run specific suite (e.g., Routing)
go test -bench=BenchmarkRouter ./benchmarks/...
```

## Interpreting Results

- **ns/op**: Nanoseconds per operation. Lower is better.
- **B/op**: Bytes allocated per operation. Lower is better.
- **allocs/op**: Number of distinct memory allocations per operation. Lower is better.

## Guardrails & Methodology

1. **Reality-based**: We benchmark typical use cases (e.g., routing with parameters and middleware), not just empty function calls.
2. **Standard Hardware**: Unless otherwise noted, official results are generated on standardized cloud instances to ensure comparability.
3. **No Cheating**: We do not use unsafe or misleading micro-optimizations that wouldn't be present in a real application.

## Current Suites

- **Routing**: Measures URL matching, parameter extraction, and middleware overhead.
- **JSON**: Measures serialization/deserialization performance using the Bytedance Sonic engine.
- **ORM**: (Coming Soon) Measures model scanning and relationship assembly.
