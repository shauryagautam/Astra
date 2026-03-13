package benchmarks

import (
	"testing"

	"github.com/astraframework/astra/json"
)

type SmallStruct struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var smallStruct = SmallStruct{ID: 1, Name: "Astra", Email: "astra@example.com"}

func BenchmarkJSON_Marshal_Sonic(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(smallStruct)
	}
}

func BenchmarkJSON_Unmarshal_Sonic(b *testing.B) {
	payload, _ := json.Marshal(smallStruct)
	var dest SmallStruct
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = json.Unmarshal(payload, &dest)
	}
}
