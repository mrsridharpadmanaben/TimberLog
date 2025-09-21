package query

import (
	"github.com/mrsridharpadmanaben/TimberLog/pkg/index"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/storage"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type QueryEngine struct {
	indexManager   *index.IndexManager
	manifest       *storage.Manifest
	segmentManager *storage.SegmentManager
}

// NewQueryEngine creates a new instance.
func NewQueryEngine(indexManager *index.IndexManager, manifest *storage.Manifest, segmentManager *storage.SegmentManager) *QueryEngine {
	return &QueryEngine{
		indexManager:   indexManager,
		manifest:       manifest,
		segmentManager: segmentManager,
	}
}

// Execute runs a query and returns results.
func (queryEngine *QueryEngine) Execute(query *Query) ([]types.LogEntry, error) {
	plan := PlanQuery(query, queryEngine.indexManager, queryEngine.manifest, queryEngine.segmentManager)
	return ExecutePlan(plan, queryEngine.segmentManager)
}
