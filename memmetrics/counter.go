package memmetrics

import (
	"fmt"
	"time"

	"github.com/mailgun/holster"
)

type rcOptSetter func(*RollingCounter) error

// CounterClock defines a counter clock
func CounterClock(c holster.Clock) rcOptSetter {
	return func(r *RollingCounter) error {
		r.clock = c
		return nil
	}
}

// RollingCounter Calculates in memory failure rate of an endpoint using rolling window of a predefined size
type RollingCounter struct {
	clock          holster.Clock
	resolution     time.Duration
	values         []int
	countedBuckets int // how many samples in different buckets have we collected so far
	lastBucket     int // last recorded bucket
	lastUpdated    time.Time
}

// NewCounter creates a counter with fixed amount of buckets that are rotated every resolution period.
// E.g. 10 buckets with 1 second means that every new second the bucket is refreshed, so it maintains 10 second rolling window.
// By default creates a bucket with 10 buckets and 1 second resolution
func NewCounter(buckets int, resolution time.Duration, options ...rcOptSetter) (*RollingCounter, error) {
	if buckets <= 0 {
		return nil, fmt.Errorf("Buckets should be >= 0")
	}
	if resolution < time.Second {
		return nil, fmt.Errorf("Resolution should be larger than a second")
	}

	rc := &RollingCounter{
		lastBucket: -1,
		resolution: resolution,

		values: make([]int, buckets),
	}

	for _, o := range options {
		if err := o(rc); err != nil {
			return nil, err
		}
	}

	if rc.clock == nil {
		rc.clock = &holster.SystemClock{}
	}

	return rc, nil
}

// Append append a counter
func (c *RollingCounter) Append(o *RollingCounter) error {
	c.Inc(int(o.Count()))
	return nil
}

// Clone clone a counter
func (c *RollingCounter) Clone() *RollingCounter {
	c.cleanup()
	other := &RollingCounter{
		resolution:  c.resolution,
		values:      make([]int, len(c.values)),
		clock:       c.clock,
		lastBucket:  c.lastBucket,
		lastUpdated: c.lastUpdated,
	}
	copy(other.values, c.values)
	return other
}

// Reset reset a counter
func (c *RollingCounter) Reset() {
	c.lastBucket = -1
	c.countedBuckets = 0
	c.lastUpdated = time.Time{}
	for i := range c.values {
		c.values[i] = 0
	}
}

// CountedBuckets gets counted buckets
func (c *RollingCounter) CountedBuckets() int {
	return c.countedBuckets
}

// Count counts
func (c *RollingCounter) Count() int64 {
	c.cleanup()
	return c.sum()
}

// Resolution gets resolution
func (c *RollingCounter) Resolution() time.Duration {
	return c.resolution
}

// Buckets gets buckets
func (c *RollingCounter) Buckets() int {
	return len(c.values)
}

// WindowSize gets windows size
func (c *RollingCounter) WindowSize() time.Duration {
	return time.Duration(len(c.values)) * c.resolution
}

// Inc increment counter
func (c *RollingCounter) Inc(v int) {
	c.cleanup()
	c.incBucketValue(v)
}

func (c *RollingCounter) incBucketValue(v int) {
	now := c.clock.Now()
	bucket := c.getBucket(now)
	c.values[bucket] += v
	c.lastUpdated = now
	// Update usage stats if we haven't collected enough data
	if c.countedBuckets < len(c.values) {
		// Only update if we have advanced to the next bucket and not incremented the value
		// in the current bucket.
		if c.lastBucket != bucket {
			c.lastBucket = bucket
			c.countedBuckets++
		}
	}
}

// Returns the number in the moving window bucket that this slot occupies
func (c *RollingCounter) getBucket(t time.Time) int {
	return int(t.Truncate(c.resolution).Unix() % int64(len(c.values)))
}

// Reset buckets that were not updated
func (c *RollingCounter) cleanup() {
	now := c.clock.Now()
	for i := 0; i < len(c.values); i++ {
		now = now.Add(time.Duration(-1*i) * c.resolution)
		if now.Truncate(c.resolution).After(c.lastUpdated.Truncate(c.resolution)) {
			c.values[c.getBucket(now)] = 0
		} else {
			break
		}
	}
}

func (c *RollingCounter) sum() int64 {
	out := int64(0)
	for _, v := range c.values {
		out += int64(v)
	}
	return out
}
