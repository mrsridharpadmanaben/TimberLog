package query

import (
	"sort"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

// ExecutePlan runs the query plan against segment files
func ExecutePlan(plan *QueryPlan, segmentManager *storage.SegmentManager) ([]types.LogEntry, error) {
	var results []types.LogEntry

	for _, segPath := range plan.Segments {
		offsets := plan.Offsets[segPath]

		// Read only matching offsets
		entries, err := segmentManager.ReadSegment(segPath, offsets)
		if err != nil {
			return nil, err
		}

		// Apply remaining filters (non-indexed or safety check)
		for _, e := range entries {
			if ApplyFilters(e, plan.Filter) {
				results = append(results, e)

				if len(results) >= plan.Query.Limit {
					break
				}
			}
		}

		if len(results) >= plan.Query.Limit {
			break
		}
	}

	// Sort by timestamp
	if len(results) > 1 {
		sort.Slice(results, func(i, j int) bool {
			if plan.Query.SortAsc {
				return results[i].Timestamp < results[j].Timestamp
			}
			return results[i].Timestamp < results[j].Timestamp
		})
	}

	// // Apply limit
	// if plan.Query.Limit > 0 && len(results) > plan.Query.Limit {
	// 	results = results[:plan.Query.Limit]
	// }

	return results, nil
}
