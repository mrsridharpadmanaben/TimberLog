package query

import (
	"path/filepath"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/index"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
)

// FilterExpression represents a single condition with logical operator
type FilterExpression struct {
	Field    string
	Value    string
	Operator string // "AND" or "OR"
}

// Query represents the high-level user request.
type Query struct {
	StartTime int64
	EndTime   int64
	Filters   []FilterExpression // flexible AND/OR filters
	Limit     int
	SortAsc   bool
}

// QueryPlan describes which segments and offsets to read
type QueryPlan struct {
	Segments []string
	Offsets  map[string][]int64
	Query    *Query
	Filter   Filter
}

// PlanQuery decides which segments and offsets to use
func PlanQuery(query *Query, indexManager *index.IndexManager, manifest *storage.Manifest, activeSegment *storage.SegmentManager) *QueryPlan {
	plan := &QueryPlan{
		Segments: []string{},
		Offsets:  make(map[string][]int64),
		Query:    query,
	}

	if plan.Query.Limit == 0 {
		plan.Query.Limit = 100
	}

	// --- Build filter tree dynamically ---
	var filterStack []Filter

	// timestamp filter
	if query.StartTime != 0 || query.EndTime != 0 {
		filterStack = append(filterStack, &TimestampFilter{
			Start: query.StartTime,
			End:   query.EndTime,
		})
	}

	// field filters
	for _, f := range query.Filters {
		newFilter := &FieldFilter{Field: f.Field, Value: f.Value}
		if f.Operator == "OR" && len(filterStack) > 0 {
			// combine last filter with OR
			last := filterStack[len(filterStack)-1]
			filterStack[len(filterStack)-1] = &OrFilter{Filters: []Filter{last, newFilter}}
		} else {
			// default AND
			filterStack = append(filterStack, newFilter)
		}
	}

	// finalize filter
	if len(filterStack) == 1 {
		plan.Filter = filterStack[0]
	} else if len(filterStack) > 1 {
		plan.Filter = &AndFilter{Filters: filterStack}
	}

	// // --- Rotated segments from manifest ---
	// for _, seg := range manifest.GetSegments() {
	// 	if seg.MaxTimestamp >= query.StartTime && seg.MinTimestamp <= query.EndTime {
	// 		path := filepath.Join(activeSegment.Dir(), seg.FileName)
	// 		plan.Segments = append(plan.Segments, path)

	// 		// Use indexes if available
	// 		offsets := getOffsetsForSegment(seg.FileName, query, indexManager)
	// 		plan.Offsets[path] = offsets
	// 	}
	// }

	// // --- Active segment ---
	// activeMeta := activeSegment.ActiveSegmentMeta()

	// if activeMeta.MaxTimestamp >= query.StartTime && activeMeta.MinTimestamp <= query.EndTime {
	// 	path := filepath.Join(activeSegment.Dir(), activeMeta.FileName)
	// 	plan.Segments = append(plan.Segments, path)

	// 	offsets := getOffsetsForSegment(activeMeta.FileName, query, indexManager)
	// 	plan.Offsets[path] = offsets
	// }

	// --- Select segments from manifest ---
	for _, seg := range manifest.GetSegments() {
		if (query.StartTime == 0 || seg.MaxTimestamp >= query.StartTime) &&
			(query.EndTime == 0 || seg.MinTimestamp <= query.EndTime) {

			path := filepath.Join(activeSegment.Dir(), seg.FileName)
			plan.Segments = append(plan.Segments, path)

			offsets := getOffsetsForSegment(query, indexManager)
			plan.Offsets[path] = offsets
		}
	}

	// --- Active segment ---
	activeMeta := activeSegment.ActiveSegmentMeta()
	if (query.StartTime == 0 || activeMeta.MaxTimestamp >= query.StartTime) &&
		(query.EndTime == 0 || activeMeta.MinTimestamp <= query.EndTime) {

		path := filepath.Join(activeSegment.Dir(), activeMeta.FileName)
		plan.Segments = append(plan.Segments, path)

		offsets := getOffsetsForSegment(query, indexManager)
		plan.Offsets[path] = offsets
	}

	return plan
}

// Helper to get offsets from indexes for a segment
func getOffsetsForSegment(query *Query, indexManager *index.IndexManager) []int64 {
	var offsets []int64

	// Try timestamp-only index first
	if idxOffsets := indexManager.Lookup("timestamp", query.StartTime, query.EndTime, ""); len(idxOffsets) > 0 {
		offsets = append(offsets, idxOffsets...)
	}

	// Apply other indexed filters (intersection)
	for _, filter := range query.Filters {
		if idxOffsets := indexManager.Lookup(filter.Field, query.StartTime, query.EndTime, filter.Value); len(idxOffsets) > 0 {
			if len(offsets) == 0 {
				offsets = idxOffsets
			} else {
				offsets = intersect(offsets, idxOffsets)
			}
		}
	}

	return offsets
}

// Simple slice intersection
func intersect(a, b []int64) []int64 {
	m := make(map[int64]struct{}, len(a))
	for _, v := range a {
		m[v] = struct{}{}
	}
	var res []int64
	for _, v := range b {
		if _, ok := m[v]; ok {
			res = append(res, v)
		}
	}
	return res
}
