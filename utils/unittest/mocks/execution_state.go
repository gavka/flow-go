package mocks

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	state "github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// ES is a mocked version of execution state that
// simulates some of its behavior for testing purpose
type ES struct {
	sync.Mutex
	state.ExecutionState
	commits map[flow.Identifier]flow.StateCommitment
}

func NewES(seal *flow.Seal) *ES {
	commits := make(map[flow.Identifier]flow.StateCommitment)
	commits[seal.BlockID] = seal.FinalState
	return &ES{
		commits: commits,
	}
}

func (es *ES) PersistStateCommitment(ctx context.Context, blockID flow.Identifier, commit flow.StateCommitment) error {
	es.Lock()
	defer es.Unlock()
	es.commits[blockID] = commit
	return nil
}

func (es *ES) StateCommitmentByBlockID(ctx context.Context, blockID flow.Identifier) (flow.StateCommitment, error) {
	commit, ok := es.commits[blockID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return commit, nil
}

func ExecuteBlock(t *testing.T, es *ES, block *flow.Block) {
	_, ok := es.commits[block.Header.ParentID]
	require.True(t, ok, "parent block not executed")
	require.NoError(t,
		es.PersistStateCommitment(
			context.Background(),
			block.ID(),
			unittest.StateCommitmentFixture()))
}