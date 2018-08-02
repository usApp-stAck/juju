// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/apiserver/common (interfaces: UpgradeSeriesUnit)

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	model "github.com/juju/juju/core/model"
	names_v2 "gopkg.in/juju/names.v2"
	reflect "reflect"
)

// MockUpgradeSeriesUnit is a mock of UpgradeSeriesUnit interface
type MockUpgradeSeriesUnit struct {
	ctrl     *gomock.Controller
	recorder *MockUpgradeSeriesUnitMockRecorder
}

// MockUpgradeSeriesUnitMockRecorder is the mock recorder for MockUpgradeSeriesUnit
type MockUpgradeSeriesUnitMockRecorder struct {
	mock *MockUpgradeSeriesUnit
}

// NewMockUpgradeSeriesUnit creates a new mock instance
func NewMockUpgradeSeriesUnit(ctrl *gomock.Controller) *MockUpgradeSeriesUnit {
	mock := &MockUpgradeSeriesUnit{ctrl: ctrl}
	mock.recorder = &MockUpgradeSeriesUnitMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockUpgradeSeriesUnit) EXPECT() *MockUpgradeSeriesUnitMockRecorder {
	return m.recorder
}

// AssignedMachineId mocks base method
func (m *MockUpgradeSeriesUnit) AssignedMachineId() (string, error) {
	ret := m.ctrl.Call(m, "AssignedMachineId")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AssignedMachineId indicates an expected call of AssignedMachineId
func (mr *MockUpgradeSeriesUnitMockRecorder) AssignedMachineId() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AssignedMachineId", reflect.TypeOf((*MockUpgradeSeriesUnit)(nil).AssignedMachineId))
}

// SetUpgradeSeriesStatus mocks base method
func (m *MockUpgradeSeriesUnit) SetUpgradeSeriesStatus(arg0 model.UnitSeriesUpgradeStatus, arg1 model.UpgradeSeriesStatusType) error {
	ret := m.ctrl.Call(m, "SetUpgradeSeriesStatus", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetUpgradeSeriesStatus indicates an expected call of SetUpgradeSeriesStatus
func (mr *MockUpgradeSeriesUnitMockRecorder) SetUpgradeSeriesStatus(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetUpgradeSeriesStatus", reflect.TypeOf((*MockUpgradeSeriesUnit)(nil).SetUpgradeSeriesStatus), arg0, arg1)
}

// Tag mocks base method
func (m *MockUpgradeSeriesUnit) Tag() names_v2.Tag {
	ret := m.ctrl.Call(m, "Tag")
	ret0, _ := ret[0].(names_v2.Tag)
	return ret0
}

// Tag indicates an expected call of Tag
func (mr *MockUpgradeSeriesUnitMockRecorder) Tag() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Tag", reflect.TypeOf((*MockUpgradeSeriesUnit)(nil).Tag))
}

// UpgradeSeriesStatus mocks base method
func (m *MockUpgradeSeriesUnit) UpgradeSeriesStatus(arg0 model.UpgradeSeriesStatusType) (model.UnitSeriesUpgradeStatus, error) {
	ret := m.ctrl.Call(m, "UpgradeSeriesStatus", arg0)
	ret0, _ := ret[0].(model.UnitSeriesUpgradeStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpgradeSeriesStatus indicates an expected call of UpgradeSeriesStatus
func (mr *MockUpgradeSeriesUnitMockRecorder) UpgradeSeriesStatus(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpgradeSeriesStatus", reflect.TypeOf((*MockUpgradeSeriesUnit)(nil).UpgradeSeriesStatus), arg0)
}
