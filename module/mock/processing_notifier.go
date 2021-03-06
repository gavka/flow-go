// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// ProcessingNotifier is an autogenerated mock type for the ProcessingNotifier type
type ProcessingNotifier struct {
	mock.Mock
}

// Notify provides a mock function with given fields: entityID
func (_m *ProcessingNotifier) Notify(entityID flow.Identifier) {
	_m.Called(entityID)
}
