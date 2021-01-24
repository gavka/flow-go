// Code generated by mockery v1.0.0. DO NOT EDIT.

package mempool

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// ResultForest is an autogenerated mock type for the ResultForest type
type ResultForest struct {
	mock.Mock
}

// Add provides a mock function with given fields: receipt
func (_m *ResultForest) Add(receipt *flow.ExecutionReceipt) bool {
	ret := _m.Called(receipt)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*flow.ExecutionReceipt) bool); ok {
		r0 = rf(receipt)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// All provides a mock function with given fields:
func (_m *ResultForest) All() []*flow.ExecutionReceipt {
	ret := _m.Called()

	var r0 []*flow.ExecutionReceipt
	if rf, ok := ret.Get(0).(func() []*flow.ExecutionReceipt); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*flow.ExecutionReceipt)
		}
	}

	return r0
}

// ByID provides a mock function with given fields: receiptID
func (_m *ResultForest) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, bool) {
	ret := _m.Called(receiptID)

	var r0 *flow.ExecutionReceipt
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.ExecutionReceipt); ok {
		r0 = rf(receiptID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionReceipt)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(receiptID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Has provides a mock function with given fields: receiptID
func (_m *ResultForest) Has(receiptID flow.Identifier) bool {
	ret := _m.Called(receiptID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(receiptID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Rem provides a mock function with given fields: receiptID
func (_m *ResultForest) Rem(receiptID flow.Identifier) bool {
	ret := _m.Called(receiptID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(receiptID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *ResultForest) Size() uint {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}