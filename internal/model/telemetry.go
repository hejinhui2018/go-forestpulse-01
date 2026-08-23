// Package model defines the canonical, transport-agnostic types used across the
// forestpulse telemetry pipeline. Types here carry no I/O or encoding
// dependencies beyond the standard library so they can be imported freely by
// collectors, exporters, and storage layers alike.
package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// MetricKind classifies the measurement a Sample carries. It drives how
// downstream layers aggregate, store, and query the value.
type MetricKind string

const (
	// MetricKindCounter is a monotonically non-decreasing cumulative value;
	// only deltas between samples are meaningful.
	MetricKindCounter MetricKind = "counter"
	// MetricKindGauge is an instantaneous point-in-time value that may rise
	// or fall between samples.
	MetricKindGauge MetricKind = "gauge"
	// MetricKindSummary captures a distributional observation (count, sum,
	// quantiles) reported as a single rolled-up value.
	MetricKindSummary MetricKind = "summary"
)

// Sample is a single telemetry observation collected from a source at a point
// in time. A Sample is the smallest unit of data that flows through the
// pipeline; multiple Samples are grouped into a Batch for transport.
type Sample struct {
	// Name is the fully-qualified metric identifier, e.g.
	// "forest.heartbeat.latency_ms". It must be non-empty and stable across
	// observations of the same logical series.
	Name string `json:"name"`

	// Value is the observed measurement. Counters report their cumulative
	// value at Timestamp; gauges report the instantaneous reading.
	Value float64 `json:"value"`

	// Kind declares how Value should be interpreted and aggregated.
	Kind MetricKind `json:"kind"`

	// Timestamp is the UTC instant the observation was recorded. Exporters
	// must not assume ordering within a Batch; use Batch.TimeRange instead.
	Timestamp time.Time `json:"timestamp"`

	// Source identifies the emitting component and deployment context.
	Source Source `json:"source"`

	// Tags is the ordered set of labels that, together with Name and Kind,
	// uniquely identify the series this Sample belongs to. Ordered (rather
	// than a map) so serialization is stable across runs.
	Tags []Tag `json:"tags,omitempty"`

	// Unit optionally qualifies Value's dimension, e.g. "ms", "bytes",
	// "ops/s". Omit for unitless counts.
	Unit string `json:"unit,omitempty"`
}

// Source identifies where a Sample originated, from the physical host up to
// the logical service and its deployment environment.
type Source struct {
	// Host is the hostname or container id of the emitting process.
	Host string `json:"host,omitempty"`

	// Service is the logical service name, e.g. "ingest-worker".
	Service string `json:"service,omitempty"`

	// Env is the deployment environment, e.g. "prod", "staging".
	Env string `json:"env,omitempty"`

	// Instance distinguishes replicas of the same Service when present.
	Instance string `json:"instance,omitempty"`
}

// Tag is an ordered key/value label attached to a Sample.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Validate reports an error if the Sample is missing required fields or has
// values that would corrupt downstream aggregation. It is intentionally
// cheap so collectors can call it on the hot path before enqueuing.
func (s Sample) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("model: sample name is required")
	}
	if s.Kind == "" {
		return fmt.Errorf("model: sample %q: kind is required", s.Name)
	}
	switch s.Kind {
	case MetricKindCounter, MetricKindGauge, MetricKindSummary:
	default:
		return fmt.Errorf("model: sample %q: unknown kind %q", s.Name, s.Kind)
	}
	if s.Timestamp.IsZero() {
		return fmt.Errorf("model: sample %q: timestamp is required", s.Name)
	}
	if s.Source.Service == "" && s.Source.Host == "" {
		return fmt.Errorf("model: sample %q: source service or host is required", s.Name)
	}
	for i, t := range s.Tags {
		if t.Key == "" {
			return fmt.Errorf("model: sample %q: tag[%d] key is required", s.Name, i)
		}
	}
	return nil
}

// Key returns a stable, dedup-safe identity for the series this Sample belongs
// to: Name, Kind, Source, and the sorted Tags. Two Samples with the same Key
// are observations of the same logical series at different Timestamps. Tags
// are sorted by Key so callers cannot perturb identity by reordering labels.
func (s Sample) Key() string {
	var b strings.Builder
	b.Grow(len(s.Name) + len(s.Kind) + 64)
	b.WriteString(string(s.Kind))
	b.WriteByte('|')
	b.WriteString(s.Name)
	b.WriteByte('|')
	b.WriteString(s.Source.Host)
	b.WriteByte('/')
	b.WriteString(s.Source.Service)
	b.WriteByte('/')
	b.WriteString(s.Source.Env)
	b.WriteByte('/')
	b.WriteString(s.Source.Instance)

	if len(s.Tags) == 0 {
		return b.String()
	}
	tags := make([]Tag, len(s.Tags))
	copy(tags, s.Tags)
	sort.Slice(tags, func(i, j int) bool { return tags[i].Key < tags[j].Key })
	b.WriteByte('|')
	for i, t := range tags {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(t.Key)
		b.WriteByte('=')
		b.WriteString(t.Value)
	}
	return b.String()
}

// Batch is a collection of Samples intended to be transmitted and processed
// together. Batching amortizes per-request overhead and lets exporters reason
// about a bounded span of time in one operation.
type Batch struct {
	// ID is a caller-assigned, transport-unique identifier for the batch. It
	// is used for at-most-once delivery and log correlation; it carries no
	// ordering semantics.
	ID string `json:"id"`

	// Samples is the ordered payload of the batch. Order reflects collection
	// order but is not semantically significant to consumers.
	Samples []Sample `json:"samples"`

	// Started is the earliest Timestamp among Samples; Ended is the latest.
	// Collectors should set them when finalizing the batch; TimeRange derives
	// the span from the payload when they are zero for safety.
	Started time.Time `json:"started,omitempty"`
	Ended   time.Time `json:"ended,omitempty"`

	// Source is the origin shared by every Sample in the batch. When non-empty,
	// it should agree with each Sample.Source.
	Source Source `json:"source,omitempty"`
}

// Len returns the number of Samples in the batch.
func (b Batch) Len() int { return len(b.Samples) }

// IsEmpty reports whether the batch carries no samples.
func (b Batch) IsEmpty() bool { return len(b.Samples) == 0 }

// TimeRange returns the [start, end) span the batch covers. When Started or
// Ended are unset it recomputes them from the payload so the result is always
// correct regardless of how the caller populated the fields.
func (b Batch) TimeRange() (start, end time.Time) {
	if !b.Started.IsZero() && !b.Ended.IsZero() {
		return b.Started, b.Ended
	}
	if len(b.Samples) == 0 {
		return
	}
	start = b.Samples[0].Timestamp
	end = start
	for _, s := range b.Samples[1:] {
		if s.Timestamp.Before(start) {
			start = s.Timestamp
		}
		if s.Timestamp.After(end) {
			end = s.Timestamp
		}
	}
	return start, end
}

// Validate reports an error if the batch itself is malformed or if any Sample
// it carries is invalid. The first offending sample short-circuits; callers
// needing the full set of failures should validate samples individually.
func (b Batch) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return errors.New("model: batch id is required")
	}
	if len(b.Samples) == 0 {
		return fmt.Errorf("model: batch %q: empty", b.ID)
	}
	for i, s := range b.Samples {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("model: batch %q: sample[%d]: %w", b.ID, i, err)
		}
	}
	if !b.Started.IsZero() && !b.Ended.IsZero() && b.Ended.Before(b.Started) {
		return fmt.Errorf("model: batch %q: ended before started", b.ID)
	}
	return nil
}

// Append adds a sample to the batch, extending Samples in place. It does not
// validate; callers should Validate the sample first on the hot path. The
// receiver is returned for chaining.
func (b *Batch) Append(s Sample) *Batch {
	b.Samples = append(b.Samples, s)
	return b
}
