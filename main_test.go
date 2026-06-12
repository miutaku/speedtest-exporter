package main

import "testing"

func TestParseServers(t *testing.T) {
	got := parseServers(" 24333,75170,, 14623 ")
	want := []string{"24333", "75170", "14623"}

	if len(got) != len(want) {
		t.Fatalf("parseServers length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseServers[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseAggregationMode(t *testing.T) {
	tests := map[string]AggregationMode{
		"best":    AggregationBest,
		"average": AggregationAverage,
		"avg":     AggregationAverage,
		"median":  AggregationMedian,
		"worst":   AggregationWorst,
		" BEST ":  AggregationBest,
	}

	for input, want := range tests {
		got, err := parseAggregationMode(input)
		if err != nil {
			t.Fatalf("parseAggregationMode(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseAggregationMode(%q) = %q, want %q", input, got, want)
		}
	}

	if _, err := parseAggregationMode("p95"); err == nil {
		t.Fatal("parseAggregationMode did not reject unsupported mode")
	}
}

func TestAggregateResults(t *testing.T) {
	results := []*SpeedTestResult{
		{Download: 100, Upload: 10, Ping: 5},
		{Download: 80, Upload: 12, Ping: 3},
		{Download: 120, Upload: 8, Ping: 7},
	}

	tests := []struct {
		name string
		mode AggregationMode
		want SpeedTestResult
	}{
		{name: "best", mode: AggregationBest, want: SpeedTestResult{Download: 120, Upload: 12, Ping: 3}},
		{name: "average", mode: AggregationAverage, want: SpeedTestResult{Download: 100, Upload: 10, Ping: 5}},
		{name: "median", mode: AggregationMedian, want: SpeedTestResult{Download: 100, Upload: 10, Ping: 5}},
		{name: "worst", mode: AggregationWorst, want: SpeedTestResult{Download: 80, Upload: 8, Ping: 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateResults(results, tt.mode)
			if *got != tt.want {
				t.Fatalf("aggregateResults() = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestMedianEvenNumberOfResults(t *testing.T) {
	results := []*SpeedTestResult{
		{Download: 100, Upload: 10, Ping: 5},
		{Download: 50, Upload: 20, Ping: 7},
	}

	got := aggregateResults(results, AggregationMedian)
	want := SpeedTestResult{Download: 75, Upload: 15, Ping: 6}
	if *got != want {
		t.Fatalf("aggregateResults() = %+v, want %+v", *got, want)
	}
}

func TestAggregateResultsEmpty(t *testing.T) {
	if got := aggregateResults(nil, AggregationBest); got != nil {
		t.Fatalf("aggregateResults(nil) = %+v, want nil", got)
	}
}
