package query

import (
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type ResultSet struct {
	Entries []types.LogEntry
	Count   int
}
