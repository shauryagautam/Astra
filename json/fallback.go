//go:build !amd64
// +build !amd64

package json

import (
	"encoding/json"
	"io"
)

var (
	Marshal       = json.Marshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
	Valid         = json.Valid
)

func MarshalString(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func UnmarshalString(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

func NewEncoder(w io.Writer) *json.Encoder {
	return json.NewEncoder(w)
}

func NewDecoder(r io.Reader) *json.Decoder {
	return json.NewDecoder(r)
}

type RawMessage = json.RawMessage
type Encoder = json.Encoder
type Decoder = json.Decoder
