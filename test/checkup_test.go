package test

import (
	"fmt"
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestComputeStats(t *testing.T) {
	s := Result{Times: []Attempt{
		{RTT: 7 * time.Second},
		{RTT: 4 * time.Second},
		{RTT: 4 * time.Second},
		{RTT: 6 * time.Second},
		{RTT: 6 * time.Second},
		{RTT: 3 * time.Second},
	}}.ComputeStats()

	if got, want := s.Total, 30*time.Second; got != want {
		t.Errorf("Expected Total=%v, got %v", want, got)
	}
	if got, want := s.Mean, 5*time.Second; got != want {
		t.Errorf("Expected Mean=%v, got %v", want, got)
	}
	if got, want := s.Median, 5*time.Second; got != want {
		t.Errorf("Expected Median=%v, got %v", want, got)
	}
	if got, want := s.Min, 3*time.Second; got != want {
		t.Errorf("Expected Min=%v, got %v", want, got)
	}
	if got, want := s.Max, 7*time.Second; got != want {
		t.Errorf("Expected Max=%v, got %v", want, got)
	}
}

func TestResultStatus(t *testing.T) {
	r := Result{Healthy: true}
	if got, want := r.Status(), Healthy; got != want {
		t.Errorf("Expected status '%s' but got: '%s'", want, got)
	}

	r = Result{Degraded: true}
	if got, want := r.Status(), Degraded; got != want {
		t.Errorf("Expected status '%s' but got: '%s'", want, got)
	}

	r = Result{Down: true}
	if got, want := r.Status(), Down; got != want {
		t.Errorf("Expected status '%s' but got: '%s'", want, got)
	}

	r = Result{}
	if got, want := r.Status(), Unknown; got != want {
		t.Errorf("Expected status '%s' but got: '%s'", want, got)
	}

	// These are invalid states, but we need to test anyway in case a
	// checker is buggy. We expect the worst of the enabled fields.
	r = Result{Down: true, Degraded: true}
	if got, want := r.Status(), Down; got != want {
		t.Errorf("(INVALID RESULT CASE) Expected status '%s' but got: '%s'", want, got)
	}
	r = Result{Degraded: true, Healthy: true}
	if got, want := r.Status(), Degraded; got != want {
		t.Errorf("(INVALID RESULT CASE) Expected status '%s' but got: '%s'", want, got)
	}
	r = Result{Down: true, Healthy: true}
	if got, want := r.Status(), Down; got != want {
		t.Errorf("(INVALID RESULT CASE) Expected status '%s' but got: '%s'", want, got)
	}
}

func TestPriorityOver(t *testing.T) {
	for i, test := range []struct {
		status   StatusText
		another  StatusText
		expected bool
	}{
		{Down, Down, false},
		{Down, Degraded, true},
		{Down, Healthy, true},
		{Down, Unknown, true},
		{Degraded, Down, false},
		{Degraded, Degraded, false},
		{Degraded, Healthy, true},
		{Degraded, Unknown, true},
		{Healthy, Down, false},
		{Healthy, Degraded, false},
		{Healthy, Healthy, false},
		{Healthy, Unknown, true},
		{Unknown, Down, false},
		{Unknown, Degraded, false},
		{Unknown, Healthy, false},
		{Unknown, Unknown, false},
	} {
		actual := test.status.PriorityOver(test.another)
		if actual != test.expected {
			t.Errorf("Test %d: Expected %s.PriorityOver(%s)=%v, but got %v",
				i, test.status, test.another, test.expected, actual)
		}
	}
}

func TestJSON(t *testing.T) {
	jsonBytes := []byte(`{"checkers":[{"type":"http","endpoint_name":"Example (HTTP)","endpoint_url":"https://schoters.com","attempts":5}],"timestamp":"0001-01-01T00:00:00Z"}`)

	var c Checkup
	err := json.Unmarshal(jsonBytes, &c)
	if err != nil {
		t.Fatalf("Error unmarshaling: %v", err)
	}
	fmt.Printf("\n")
	fmt.Println("TestJSON")
	jsonMap := make(map[string]interface{})	
	fmt.Printf("%v\n\n", jsonMap)

	result, err := json.Marshal(c)
	// jsonData, err := json.MarshalIndent(c, "", "    ")
	// fmt.Printf("%v\n\n", string(jsonData))
	
	if err != nil {
		t.Fatalf("Error marshaling: %v", err)
	}

	if !bytes.Equal(result, jsonBytes) {
		t.Errorf("\nGot:  %s\nWant: %s", string(result), string(jsonBytes))
	}
}

var errTest = errors.New("i'm an error")

type fake struct {
	returnErr  bool
	checked    int
	stored     []Result
	maintained int
	notified   int
}

func (f *fake) Check() (Result, error) {
	f.checked++
	r := Result{Timestamp: time.Now().UTC().UnixNano()}
	if f.returnErr {
		return r, errTest
	}
	return r, nil
}

func (f *fake) Store(results []Result) error {
	f.stored = results
	if f.returnErr {
		return errTest
	}
	return nil
}

func (f *fake) Maintain() error {
	f.maintained++
	return nil
}

func (f *fake) Notify(results []Result) error {
	f.notified++
	return nil
}
