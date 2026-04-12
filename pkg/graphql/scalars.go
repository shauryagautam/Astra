package graphql

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DateTime is a custom scalar for time.Time values in GraphQL.
type DateTime struct {
	time.Time
}

// ImplementsGraphQLType maps this Go type to the GraphQL scalar type.
func (DateTime) ImplementsGraphQLType(name string) bool {
	return name == "DateTime"
}

// UnmarshalGraphQL parses a DateTime from an input value.
func (d *DateTime) UnmarshalGraphQL(input any) error {
	switch v := input.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return err
		}
		d.Time = t
		return nil
	case time.Time:
		d.Time = v
		return nil
	default:
		return fmt.Errorf("time should be a RFC3339 string")
	}
}

// MarshalJSON serializes a DateTime to JSON as an RFC3339 string.
func (d DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Time.Format(time.RFC3339))
}

// UUID is a custom scalar for uuid.UUID values in GraphQL.
type UUID struct {
	uuid.UUID
}

// ImplementsGraphQLType maps this Go type to the GraphQL scalar type.
func (UUID) ImplementsGraphQLType(name string) bool {
	return name == "UUID"
}

// UnmarshalGraphQL parses a UUID from an input value.
func (u *UUID) UnmarshalGraphQL(input any) error {
	switch v := input.(type) {
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return err
		}
		u.UUID = parsed
		return nil
	default:
		return fmt.Errorf("id should be a UUID string")
	}
}

// MarshalJSON serializes a UUID to JSON.
func (u UUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.UUID.String())
}

// JSON is a custom scalar for arbitrary JSON values in GraphQL.
type JSON struct {
	Value any
}

// ImplementsGraphQLType maps this Go type to the GraphQL scalar type.
func (JSON) ImplementsGraphQLType(name string) bool {
	return name == "JSON"
}

// UnmarshalGraphQL parses a JSON value from an input.
func (j *JSON) UnmarshalGraphQL(input any) error {
	j.Value = input
	return nil
}

// MarshalJSON serializes the JSON scalar to JSON.
func (j JSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Value)
}
