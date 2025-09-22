package query

import (
	"strings"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type Filter interface {
	Apply(entry types.LogEntry) bool
}

// FieldFilter checks fixed fields or dynamic Properties
type FieldFilter struct {
	Field string
	Value string
}

func (f *FieldFilter) Apply(entry types.LogEntry) bool {
	switch strings.ToLower(f.Field) {
	case "level":
		return string(entry.Level) == f.Value
	case "service":
		return entry.Service == f.Value
	case "host":
		return entry.Host == f.Value
	case "message":
		return strings.Contains(entry.Message, f.Value)
	case "stacktrace":
		return strings.Contains(entry.StackTrace, f.Value)
	default:
		propVal, ok := entry.Properties[f.Field]
		return ok && propVal == f.Value
	}
}

// TimestampFilter for range queries
type TimestampFilter struct {
	Start int64
	End   int64
}

func (t *TimestampFilter) Apply(entry types.LogEntry) bool {
	if t.Start != 0 && entry.Timestamp < t.Start {
		return false
	}
	if t.End != 0 && entry.Timestamp > t.End {
		return false
	}
	return true
}

// AndFilter combines multiple filters with AND logic
type AndFilter struct {
	Filters []Filter
}

func (a *AndFilter) Apply(entry types.LogEntry) bool {
	for _, f := range a.Filters {
		if !f.Apply(entry) {
			return false
		}
	}
	return true
}

// OrFilter combines multiple filters with OR logic
type OrFilter struct {
	Filters []Filter
}

func (o *OrFilter) Apply(entry types.LogEntry) bool {
	for _, f := range o.Filters {
		if f.Apply(entry) {
			return true
		}
	}
	return false
}

// ApplyFilters helper
func ApplyFilters(entry types.LogEntry, filter Filter) bool {
	if filter == nil {
		return true
	}
	return filter.Apply(entry)
}
