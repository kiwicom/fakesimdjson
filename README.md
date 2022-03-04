fakesimdjson builds a simdjson-go tape using the stdlib's JSON parser.

It is slow and does a lot of allocations.
This is a workaround to run programs using simdjson-go on developer's machine with other architecture
than amd64 (like the M1 MacBook) until simdjson-go has a arm64 or a generic version, see
https://github.com/minio/simdjson-go/issues/51 for details.

Usage:

```go
if simdjson.SupportedCPU() {
	parsed, err = simdjson.Parse(data, reused)
} else {
	parsed, err = fakesimdjson.Parse(data)
}
```
