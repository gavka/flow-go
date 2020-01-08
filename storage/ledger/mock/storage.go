// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import ledger "github.com/dapperlabs/flow-go/storage/ledger"
import mock "github.com/stretchr/testify/mock"

// Storage is an autogenerated mock type for the Storage type
type Storage struct {
	mock.Mock
}

// GetRegisters provides a mock function with given fields: registerIDs, stateCommitment
func (_m *Storage) GetRegisters(registerIDs [][]byte, stateCommitment ledger.StateCommitment) ([][]byte, error) {
	ret := _m.Called(registerIDs, stateCommitment)

	var r0 [][]byte
	if rf, ok := ret.Get(0).(func([][]byte, ledger.StateCommitment) [][]byte); ok {
		r0 = rf(registerIDs, stateCommitment)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([][]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func([][]byte, ledger.StateCommitment) error); ok {
		r1 = rf(registerIDs, stateCommitment)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRegistersWithProof provides a mock function with given fields: registerIDs, stateCommitment
func (_m *Storage) GetRegistersWithProof(registerIDs [][]byte, stateCommitment ledger.StateCommitment) ([][]byte, [][]byte, error) {
	ret := _m.Called(registerIDs, stateCommitment)

	var r0 [][]byte
	if rf, ok := ret.Get(0).(func([][]byte, ledger.StateCommitment) [][]byte); ok {
		r0 = rf(registerIDs, stateCommitment)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([][]byte)
		}
	}

	var r1 [][]byte
	if rf, ok := ret.Get(1).(func([][]byte, ledger.StateCommitment) [][]byte); ok {
		r1 = rf(registerIDs, stateCommitment)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([][]byte)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func([][]byte, ledger.StateCommitment) error); ok {
		r2 = rf(registerIDs, stateCommitment)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// UpdateRegisters provides a mock function with given fields: registerIDs, values
func (_m *Storage) UpdateRegisters(registerIDs [][]byte, values [][]byte) (ledger.StateCommitment, error) {
	ret := _m.Called(registerIDs, values)

	var r0 ledger.StateCommitment
	if rf, ok := ret.Get(0).(func([][]byte, [][]byte) ledger.StateCommitment); ok {
		r0 = rf(registerIDs, values)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(ledger.StateCommitment)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func([][]byte, [][]byte) error); ok {
		r1 = rf(registerIDs, values)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateRegistersWithProof provides a mock function with given fields: registerIDs, values
func (_m *Storage) UpdateRegistersWithProof(registerIDs [][]byte, values [][]byte) (ledger.StateCommitment, [][]byte, error) {
	ret := _m.Called(registerIDs, values)

	var r0 ledger.StateCommitment
	if rf, ok := ret.Get(0).(func([][]byte, [][]byte) ledger.StateCommitment); ok {
		r0 = rf(registerIDs, values)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(ledger.StateCommitment)
		}
	}

	var r1 [][]byte
	if rf, ok := ret.Get(1).(func([][]byte, [][]byte) [][]byte); ok {
		r1 = rf(registerIDs, values)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([][]byte)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func([][]byte, [][]byte) error); ok {
		r2 = rf(registerIDs, values)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}
