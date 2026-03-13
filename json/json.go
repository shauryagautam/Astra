//go:build amd64
// +build amd64

package json

import (
	"encoding/json"
	"io"

	"github.com/bytedance/sonic"
)

var (
	ConfigDefault   = sonic.ConfigDefault
	Marshal         = ConfigDefault.Marshal
	Unmarshal       = ConfigDefault.Unmarshal
	MarshalIndent   = ConfigDefault.MarshalIndent
	MarshalString   = ConfigDefault.MarshalToString
	UnmarshalString = ConfigDefault.UnmarshalFromString
	Valid           = ConfigDefault.Valid
)

func NewEncoder(w io.Writer) Encoder {
	return ConfigDefault.NewEncoder(w)
}

func NewDecoder(r io.Reader) Decoder {
	return ConfigDefault.NewDecoder(r)
}

// MarshalToString marshals data to JSON string using Sonic for ultra-fast performance
func MarshalToString(v any) (string, error) {
	return ConfigDefault.MarshalToString(v)
}

type Encoder = sonic.Encoder
type Decoder = sonic.Decoder
type RawMessage = json.RawMessage
