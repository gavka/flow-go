package testutil

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

func DeployCounterContractTransaction() flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(`
		access(all) contract Container {
			access(all) resource Counter {
				pub var count: Int
	
				init(_ v: Int) {
					self.count = v
				}
				pub fun add(_ count: Int) {
					self.count = self.count + count
				}
			}
			pub fun createCounter(_ v: Int): @Counter {
				return <-create Counter(v)
			}
		}`))

	return flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }`, encoded)),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}
}

func CreateCounterTransaction() flow.TransactionBody {
	return flow.TransactionBody{
		Script: []byte(`
			import 0x01
			
			transaction {
				prepare(acc: AuthAccount) {
					var maybeCounter <- acc.load<@Container.Counter>(from: /storage/counter)
			
					if maybeCounter == nil {
						maybeCounter <-! Container.createCounter(3)		
					}
			
					acc.save(<-maybeCounter!, to: /storage/counter)
				}   	
			}`),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}
}

func AddToCounterTransaction() flow.TransactionBody {
	return flow.TransactionBody{
		Script: []byte(`
			import 0x01
			
			transaction {
				prepare(acc: AuthAccount) {
					let counter <- acc.load<@Container.Counter>(from: /storage/counter)
			
					counter?.add(2)
			
					acc.save(<-counter, to: /storage/counter)
				}
			}`),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}
}
