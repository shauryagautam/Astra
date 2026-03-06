package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
)

// MarshalDateTime serializes a time.Time to an ISO8601 string.
func MarshalDateTime(t time.Time) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		io.WriteString(w, fmt.Sprintf("%q", t.Format(time.RFC3339)))
	})
}

// UnmarshalDateTime deserializes an ISO8601 string to a time.Time.
func UnmarshalDateTime(v any) (time.Time, error) {
	if s, ok := v.(string); ok {
		return time.Parse(time.RFC3339, s)
	}
	return time.Time{}, fmt.Errorf("time should be a RFC3339 string")
}

// MarshalUUID serializes a uuid.UUID to a string.
func MarshalUUID(u uuid.UUID) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		io.WriteString(w, fmt.Sprintf("%q", u.String()))
	})
}

// UnmarshalUUID deserializes a string to a uuid.UUID.
func UnmarshalUUID(v any) (uuid.UUID, error) {
	if s, ok := v.(string); ok {
		return uuid.Parse(s)
	}
	return uuid.Nil, fmt.Errorf("id should be a UUID string")
}

// MarshalJSON serializes any value to a JSON-ready any.
func MarshalJSON(v any) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		bytes, _ := json.Marshal(v)
		w.Write(bytes)
	})
}

// UnmarshalJSON deserializes any JSON-ready value to any.
func UnmarshalJSON(v any) (any, error) {
	return v, nil
}
