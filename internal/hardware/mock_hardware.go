// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/project-flotta/flotta-device-worker/internal/hardware (interfaces: Hardware)

// Package hardware is a generated GoMock package.
package hardware

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	models "github.com/project-flotta/flotta-operator/models"
)

// MockHardware is a mock of Hardware interface.
type MockHardware struct {
	ctrl     *gomock.Controller
	recorder *MockHardwareMockRecorder
}

// MockHardwareMockRecorder is the mock recorder for MockHardware.
type MockHardwareMockRecorder struct {
	mock *MockHardware
}

// NewMockHardware creates a new mock instance.
func NewMockHardware(ctrl *gomock.Controller) *MockHardware {
	mock := &MockHardware{ctrl: ctrl}
	mock.recorder = &MockHardwareMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHardware) EXPECT() *MockHardwareMockRecorder {
	return m.recorder
}

// CreateHardwareMutableInformation mocks base method.
func (m *MockHardware) CreateHardwareMutableInformation() (*models.HardwareInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateHardwareMutableInformation")
	ret0, _ := ret[0].(*models.HardwareInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateHardwareMutableInformation indicates an expected call of CreateHardwareMutableInformation.
func (mr *MockHardwareMockRecorder) CreateHardwareMutableInformation() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateHardwareMutableInformation", reflect.TypeOf((*MockHardware)(nil).CreateHardwareMutableInformation))
}

// GetHardwareImmutableInformation mocks base method.
func (m *MockHardware) GetHardwareImmutableInformation(arg0 *models.HardwareInfo) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHardwareImmutableInformation", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetHardwareImmutableInformation indicates an expected call of GetHardwareImmutableInformation.
func (mr *MockHardwareMockRecorder) GetHardwareImmutableInformation(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHardwareImmutableInformation", reflect.TypeOf((*MockHardware)(nil).GetHardwareImmutableInformation), arg0)
}

// GetHardwareInformation mocks base method.
func (m *MockHardware) GetHardwareInformation() (*models.HardwareInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHardwareInformation")
	ret0, _ := ret[0].(*models.HardwareInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetHardwareInformation indicates an expected call of GetHardwareInformation.
func (mr *MockHardwareMockRecorder) GetHardwareInformation() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHardwareInformation", reflect.TypeOf((*MockHardware)(nil).GetHardwareInformation))
}

// GetMutableHardwareInfoDelta mocks base method.
func (m *MockHardware) GetMutableHardwareInfoDelta(arg0, arg1 models.HardwareInfo) *models.HardwareInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMutableHardwareInfoDelta", arg0, arg1)
	ret0, _ := ret[0].(*models.HardwareInfo)
	return ret0
}

// GetMutableHardwareInfoDelta indicates an expected call of GetMutableHardwareInfoDelta.
func (mr *MockHardwareMockRecorder) GetMutableHardwareInfoDelta(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMutableHardwareInfoDelta", reflect.TypeOf((*MockHardware)(nil).GetMutableHardwareInfoDelta), arg0, arg1)
}