package virtualmachine_test

import (
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()
	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	t.Run("transaction execution results in cached program", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Authorizers: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `),
		}
		ledger := make(virtualmachine.MapLedger)
		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)

		// Determine location of transaction
		txID := tx.ID()
		location := runtime.TransactionLocation(txID[:])

		// Get cache
		astCache, ok := vm.(virtualmachine.ASTCache)
		assert.True(t, ok)

		// Get cached program
		program, err := astCache.GetProgram(location)
		assert.NotNil(t, program)
		assert.NoError(t, err)
	})

}

func TestScriptASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()
	vm, err := virtualmachine.New(rt)

	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	t.Run("script execution results in cached program", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := make(virtualmachine.MapLedger)
		result, err := bc.ExecuteScript(ledger, script)
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Determine location
		scriptHash := hash.DefaultHasher.ComputeHash(script)
		location := runtime.ScriptLocation(scriptHash)

		// Get cache
		astCache, ok := vm.(virtualmachine.ASTCache)
		assert.True(t, ok)

		// Get cached program
		program, err := astCache.GetProgram(location)
		assert.NotNil(t, program)
		assert.NoError(t, err)

	})
}
