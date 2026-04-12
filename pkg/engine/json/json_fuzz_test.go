//go:build ignore
// +build ignore

// This file is excluded from the standard build because the json package
// uses sonic (amd64-only build tag). Fuzz tests run on amd64 only.
// Run with: GOARCH=amd64 go test -fuzz=FuzzUnmarshal ./json/ -fuzztime=30s

package json_test

import (
	stdjson "encoding/json"
	"testing"

	asjson "github.com/shauryagautam/Astra/pkg/engine/json"
)

// FuzzUnmarshal verifies Astra's JSON unmarshaller never panics on arbitrary input.
func FuzzUnmarshal(f *test_util.F) {
	// Seed corpus
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`"hello"`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"key":"value","num":42,"bool":true,"null":null}`))
	f.Add([]byte(`{"a":{"b":{"c":{"d":"deep"}}}}`))
	f.Add([]byte(`[1,2,3,4,5]`))
	f.Add([]byte(`{"\u0000":"\u0000"}`))   // null bytes
	f.Add([]byte(`{"a":` + string(make([]byte, 65536)) + `}`)) // oversized
	f.Add([]byte(`truetruetrue`))            // concatenated primitives
	f.Add([]byte(`{"a":1e999}`))             // out-of-range float

	f.Fuzz(func(t *testing.T, data []byte) {
		var v any
		// Must not panic
		_ = asjson.Unmarshal(data, &v)
	})
}

// FuzzMarshalRoundtrip verifies that Marshal(Unmarshal(x)) doesn't panic.
func FuzzMarshalRoundtrip(f *test_util.F) {
	f.Add([]byte(`{"id":1,"name":"Alice"}`))
	f.Add([]byte(`[1,"two",3.0,null,true]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var v any
		if err := stdjson.Unmarshal(data, &v); err != nil {
			return // Only fuzz valid JSON for roundtrip
		}
		encoded, err := asjson.Marshal(v)
		if err != nil {
			return
		}
		var v2 any
		_ = asjson.Unmarshal(encoded, &v2)
	})
}
