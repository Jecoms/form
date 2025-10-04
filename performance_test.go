package form

import (
	"fmt"
	"net/url"
	"testing"
	"time"
)

func TestNestedDecodePerformance(t *testing.T) {
	type NestedBar struct {
		Bazs   []string          `form:"bazs"`
		Lookup map[string]string `form:"lookup"`
	}

	type NestedFoo struct {
		Bars []*NestedBar `form:"bars"`
	}

	type FormRequest struct {
		Foos []*NestedFoo `form:"foos"`
	}

	decoder := NewDecoder()

	// Test with 200 values - goal is to decode in < 5ms
	numValues := 200
	urlValues := make(url.Values)

	// Generate test data with nested structure
	for i := 0; i < numValues; i++ {
		urlValues.Add(fmt.Sprintf("foos[0].bars[%d].bazs", i), fmt.Sprintf("value%d", i))
		urlValues.Add(fmt.Sprintf("foos[0].bars[%d].lookup[A]", i), fmt.Sprintf("lookupA%d", i))
	}

	var req FormRequest
	start := time.Now()
	err := decoder.Decode(&req, urlValues)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Verify correct decoding
	if len(req.Foos) != 1 {
		t.Errorf("Expected 1 Foo, got %d", len(req.Foos))
	}
	if len(req.Foos[0].Bars) != numValues {
		t.Errorf("Expected %d Bars, got %d", numValues, len(req.Foos[0].Bars))
	}

	t.Logf("Decoded %d nested values in: %v", numValues, elapsed)
	
	// Goal: < 5ms for 200 values
	maxTime := 5 * time.Millisecond
	if elapsed > maxTime {
		t.Logf("WARNING: Performance goal not met. Target: %v, Actual: %v", maxTime, elapsed)
	} else {
		t.Logf("✓ Performance goal achieved!")
	}
}

func BenchmarkNestedDecode200(b *testing.B) {
	type NestedBar struct {
		Bazs   []string          `form:"bazs"`
		Lookup map[string]string `form:"lookup"`
	}

	type NestedFoo struct {
		Bars []*NestedBar `form:"bars"`
	}

	type FormRequest struct {
		Foos []*NestedFoo `form:"foos"`
	}

	decoder := NewDecoder()
	urlValues := make(url.Values)

	for i := 0; i < 200; i++ {
		urlValues.Add(fmt.Sprintf("foos[0].bars[%d].bazs", i), fmt.Sprintf("value%d", i))
		urlValues.Add(fmt.Sprintf("foos[0].bars[%d].lookup[A]", i), fmt.Sprintf("lookupA%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		var req FormRequest
		if err := decoder.Decode(&req, urlValues); err != nil {
			b.Fatal(err)
		}
	}
}
