// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// Blocks is an autogenerated mock type for the Blocks type
type Blocks struct {
	mock.Mock
}

// ByHeightFrom provides a mock function with given fields: height, header
func (_m *Blocks) ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error) {
	ret := _m.Called(height, header)

	var r0 *flow.Header
	if rf, ok := ret.Get(0).(func(uint64, *flow.Header) *flow.Header); ok {
		r0 = rf(height, header)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(uint64, *flow.Header) error); ok {
		r1 = rf(height, header)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
