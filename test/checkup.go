// Package checkup provides means for checking and reporting the
// status and performance of various endpoints in a distributed,
// lock-free, self-hosted fashion.
package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// Checkup performs a routine checkup on endpoints or
// services.
type Checkup struct {
	// Checkers is the list of Checkers to use with
	// which to perform checks.
	Checkers []Checker `json:"checkers,omitempty"`

	// ConcurrentChecks is how many checks, at most, to
	// perform concurrently. Default is
	// DefaultConcurrentChecks.
	ConcurrentChecks int `json:"concurrent_checks,omitempty"`

	// Timestamp is the timestamp to force for all checks.
	// Useful if wanting to perform distributed check
	// "at the same time" even if they might actually
	// be a few milliseconds or seconds apart.
	Timestamp time.Time `json:"timestamp,omitempty"`

}

// Check performs the health checks. An error is only
// returned in the case of a misconfiguration or if
// any one of the Checkers returns an error.
func (c Checkup) Check() ([]Result, error) {
	
	if c.ConcurrentChecks == 0 {
		c.ConcurrentChecks = DefaultConcurrentChecks
	}
	if c.ConcurrentChecks < 0 {
		return nil, fmt.Errorf("invalid value for ConcurrentChecks: %d (must be set > 0)",
			c.ConcurrentChecks)
	}

	results := make([]Result, len(c.Checkers))
	errs := make(Errors, len(c.Checkers))
	throttle := make(chan struct{}, c.ConcurrentChecks)
	wg := sync.WaitGroup{}

	for i, checker := range c.Checkers {
		throttle <- struct{}{}
		wg.Add(1)
		go func(i int, checker Checker) {
			results[i], errs[i] = checker.Check()
			<-throttle
			wg.Done()
		}(i, checker)
	}
	wg.Wait()

	if !c.Timestamp.IsZero() {
		for i := range results {
			results[i].Timestamp = c.Timestamp.UTC().UnixNano()
		}
	}

	if !errs.Empty() {
		return results, errs
	}


	return results, nil
}

// CheckAndStore performs health checks and immediately
// stores the results to the configured storage if there
// were no errors. Checks are not performed if c.Storage
// is nil. If c.Storage is also a Maintainer, Maintain()
// will be called if Store() is successful.
func (c Checkup) CheckAndStore() error {
	
	return nil
}

// CheckAndStoreEvery calls CheckAndStore every interval. It returns
// the ticker that it's using so you can stop it when you don't want
// it to run anymore. This function does NOT block (it runs the ticker
// in a goroutine). Any errors are written to the standard logger. It
// would not be wise to set an interval lower than the time it takes
// to perform the checks.
func (c Checkup) CheckAndStoreEvery(interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := c.CheckAndStore(); err != nil {
				log.Println(err)
			}
		}
	}()
	return ticker
}

func (c Checkup) IntervalRun(interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			fmt.Println("TestTicker");
		}
	}()
	return ticker
}

// MarshalJSON marshals c into JSON with type information
// included on the interface values.
func (c Checkup) MarshalJSON() ([]byte, error) {
	// Start with the fields of c that don't require special
	// handling; unfortunately this has to mimic c's definition.
	easy := struct {
		ConcurrentChecks int       `json:"concurrent_checks,omitempty"`
		Timestamp        time.Time `json:"timestamp,omitempty"`
	}{
		ConcurrentChecks: c.ConcurrentChecks,
		Timestamp:        c.Timestamp,
	}
	result, err := json.Marshal(easy)
	if err != nil {
		return result, err
	}

	wrap := func(key string, value []byte) {
		b := append([]byte{result[0]}, []byte(`"`+key+`":`)...)
		b = append(b, value...)
		if len(result) > 2 {
			b = append(b, ',')
		}
		result = append(b, result[1:]...)
	}

	// Checkers
	if len(c.Checkers) > 0 {
		var checkers [][]byte
		for _, ch := range c.Checkers {
			chb, err := json.Marshal(ch)
			if err != nil {
				return result, err
			}
			var typeName string
			switch ch.(type) {
			case HTTPChecker:
				typeName = "http"			
			default:
				return result, fmt.Errorf("unknown Checker type")
			}
			chb = []byte(fmt.Sprintf(`{"type":"%s",%s`, typeName, string(chb[1:])))
			checkers = append(checkers, chb)
		}

		allCheckers := []byte{'['}
		allCheckers = append([]byte{'['}, bytes.Join(checkers, []byte(","))...)
		allCheckers = append(allCheckers, ']')
		wrap("checkers", allCheckers)
	}

	return result, nil
}

// UnmarshalJSON unmarshales b into c. To succeed, it
// requires type information for the interface values.
func (c *Checkup) UnmarshalJSON(b []byte) error {
	// Unmarshal as much of b as we can; this requires
	// a type that doesn't implement json.Unmarshaler,
	// hence the conversion. We also know that the
	// interface types will ultimately cause an error,
	// but we can ignore it because we handle it below.
	type checkup2 *Checkup
	json.Unmarshal(b, checkup2(c))
	c.Checkers = []Checker{} // clean the slate

	// Begin unmarshaling interface values by
	// collecting the raw JSON
	raw := struct {
		Checkers []json.RawMessage `json:"checkers"`
		
	}{}
	err := json.Unmarshal([]byte(b), &raw)
	if err != nil {
		return err
	}

	// Then collect the concrete type information
	types := struct {
		Checkers []struct {
			Type string `json:"type"`
		}
		
	}{}
	err = json.Unmarshal([]byte(b), &types)
	if err != nil {
		return err
	}

	// Finally, we unmarshal the remaining values using type
	// assertions with the help of the type information
	for i, t := range types.Checkers {
		switch t.Type {
		case "http":
			var checker HTTPChecker
			err = json.Unmarshal(raw.Checkers[i], &checker)
			if err != nil {
				return err
			}
			fmt.Println("HTTPCHECKER")
			fmt.Printf("%v", checker)
			c.Checkers = append(c.Checkers, checker)
		
		default:
			return fmt.Errorf("%s: unknown Checker type", t.Type)
		}
	}

	return nil
}

// Checker can create a Result.
type Checker interface {
	Check() (Result, error)
}

// DefaultConcurrentChecks is how many checks,
// at most, to perform concurrently.
var DefaultConcurrentChecks = 5

// FilenameFormatString is the format string used
// by GenerateFilename to create a filename.
const FilenameFormatString = "%d-check.json"

// Timestamp returns the UTC Unix timestamp in
// nanoseconds.
func Timestamp() int64 {
	return time.Now().UTC().UnixNano()
}

// GenerateFilename returns a filename that is ideal
// for storing the results file on a storage provider
// that relies on the filename for retrieval that is
// sorted by date/timeframe. It returns a string pointer
// to be used by the AWS SDK...
func GenerateFilename() *string {
	s := fmt.Sprintf(FilenameFormatString, Timestamp())
	return &s
}

// Result is the result of a health check.
type Result struct {
	// Title is the title (or name) of the thing that was checked.
	// It should be unique, as it acts like an identifier to users.
	Title string `json:"title,omitempty"`

	// Endpoint is the URL/address/path/identifier/locator/whatever
	// of what was checked.
	Endpoint string `json:"endpoint,omitempty"`

	// Timestamp is when the check occurred; UTC UnixNano format.
	Timestamp int64 `json:"timestamp,omitempty"`

	// Times is a list of each individual check attempt.
	Times Attempts `json:"times,omitempty"`

	// ThresholdRTT is the maximum RTT that was tolerated before
	// considering performance to be degraded. Leave 0 if irrelevant.
	ThresholdRTT time.Duration `json:"threshold,omitempty"`

	// Healthy, Degraded, and Down contain the ultimate conclusion
	// about the endpoint. Exactly one of these should be true;
	// any more or less is a bug.
	Healthy  bool `json:"healthy,omitempty"`
	Degraded bool `json:"degraded,omitempty"`
	Down     bool `json:"down,omitempty"`

	// Notice contains a description of some condition of this
	// check that might have affected the result in some way.
	// For example, that the median RTT is above the threshold.
	Notice string `json:"notice,omitempty"`

	// Message is an optional message to show on the status page.
	// For example, what you're doing to fix a problem.
	Message string `json:"message,omitempty"`
}

// ComputeStats computes basic statistics about r.
func (r Result) ComputeStats() Stats {
	var s Stats
	fmt.Println("COMPUTESTATS")
	fmt.Printf("%v\n",r.Times)
	for _, a := range r.Times {
		s.Total += a.RTT
		if a.RTT < s.Min || s.Min == 0 {
			s.Min = a.RTT
		}
		if a.RTT > s.Max || s.Max == 0 {
			s.Max = a.RTT
		}
	}
	sorted := make(Attempts, len(r.Times))
	copy(sorted, r.Times)
	sort.Sort(sorted)

	half := len(sorted) / 2
	if len(sorted)%2 == 0 {
		s.Median = (sorted[half-1].RTT + sorted[half].RTT) / 2
	} else {
		s.Median = sorted[half].RTT
	}

	s.Mean = time.Duration(int64(s.Total) / int64(len(r.Times)))

	return s
}

// String returns a human-readable rendering of r.
func (r Result) String() string {
	stats := r.ComputeStats()
	fmt.Println("STRING_RENDER")
	s := fmt.Sprintf("== %s - %s\n", r.Title, r.Endpoint)
	s += fmt.Sprintf("  Threshold: %s\n", r.ThresholdRTT)
	s += fmt.Sprintf("        Max: %s\n", stats.Max)
	s += fmt.Sprintf("        Min: %s\n", stats.Min)
	s += fmt.Sprintf("     Median: %s\n", stats.Median)
	s += fmt.Sprintf("       Mean: %s\n", stats.Mean)
	s += fmt.Sprintf("        All: %v\n", r.Times)
	statusLine := fmt.Sprintf(" Assessment: %v\n", r.Status())
	switch r.Status() {
	case Healthy:
		statusLine = color.GreenString(statusLine)
	case Degraded:
		statusLine = color.YellowString(statusLine)
	case Down:
		statusLine = color.RedString(statusLine)
	}
	s += statusLine
	return s
}

// Status returns a text representation of the overall status
// indicated in r.
func (r Result) Status() StatusText {
	if r.Down {
		return Down
	} else if r.Degraded {
		return Degraded
	} else if r.Healthy {
		return Healthy
	}
	return Unknown
}

// DisableColor disables ANSI colors in the Result default string.
func DisableColor() {
	color.NoColor = true
}

// StatusText is the textual representation of the
// result of a status check.
type StatusText string

// PriorityOver returns whether s has priority over other.
// For example, a Down status has priority over Degraded.
func (s StatusText) PriorityOver(other StatusText) bool {
	if s == other {
		return false
	}
	switch s {
	case Down:
		return true
	case Degraded:
		if other == Down {
			return false
		}
		return true
	case Healthy:
		if other == Unknown {
			return true
		}
		return false
	}
	return false
}

// Text representations for the status of a check.
const (
	Healthy  StatusText = "healthy"
	Degraded StatusText = "degraded"
	Down     StatusText = "down"
	Unknown  StatusText = "unknown"
)

// Attempt is an attempt to communicate with the endpoint.
type Attempt struct {
	RTT   time.Duration `json:"rtt"`
	Error string        `json:"error,omitempty"`
}

// Attempts is a list of Attempt that can be sorted by RTT.
type Attempts []Attempt

func (a Attempts) Len() int           { return len(a) }
func (a Attempts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Attempts) Less(i, j int) bool { return a[i].RTT < a[j].RTT }

// Stats is a type that holds information about a Result,
// especially its various Attempts.
type Stats struct {
	Total  time.Duration `json:"total,omitempty"`
	Mean   time.Duration `json:"mean,omitempty"`
	Median time.Duration `json:"median,omitempty"`
	Min    time.Duration `json:"min,omitempty"`
	Max    time.Duration `json:"max,omitempty"`
}

// Errors is an error type that concatenates multiple errors.
type Errors []error

// Error returns a string containing all the errors in e.
func (e Errors) Error() string {
	var errs []string
	for _, err := range e {
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	return strings.Join(errs, "; ")
}

// Empty returns whether e has any non-nil errors in it.
func (e Errors) Empty() bool {
	for _, err := range e {
		if err != nil {
			return false
		}
	}
	return true
}
