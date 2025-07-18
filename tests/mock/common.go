// Code generated by MockGen. DO NOT EDIT.
// Source: ./utils/common/interface.go

// Package mock_awsctl is a generated GoMock package.
package mock_awsctl

import (
	context "context"
	io "io"
	os "os"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockFileSystemInterface is a mock of FileSystemInterface interface.
type MockFileSystemInterface struct {
	ctrl     *gomock.Controller
	recorder *MockFileSystemInterfaceMockRecorder
}

// MockFileSystemInterfaceMockRecorder is the mock recorder for MockFileSystemInterface.
type MockFileSystemInterfaceMockRecorder struct {
	mock *MockFileSystemInterface
}

// NewMockFileSystemInterface creates a new mock instance.
func NewMockFileSystemInterface(ctrl *gomock.Controller) *MockFileSystemInterface {
	mock := &MockFileSystemInterface{ctrl: ctrl}
	mock.recorder = &MockFileSystemInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFileSystemInterface) EXPECT() *MockFileSystemInterfaceMockRecorder {
	return m.recorder
}

// MkdirAll mocks base method.
func (m *MockFileSystemInterface) MkdirAll(path string, perm os.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MkdirAll", path, perm)
	ret0, _ := ret[0].(error)
	return ret0
}

// MkdirAll indicates an expected call of MkdirAll.
func (mr *MockFileSystemInterfaceMockRecorder) MkdirAll(path, perm interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MkdirAll", reflect.TypeOf((*MockFileSystemInterface)(nil).MkdirAll), path, perm)
}

// ReadFile mocks base method.
func (m *MockFileSystemInterface) ReadFile(name string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadFile", name)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadFile indicates an expected call of ReadFile.
func (mr *MockFileSystemInterfaceMockRecorder) ReadFile(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadFile", reflect.TypeOf((*MockFileSystemInterface)(nil).ReadFile), name)
}

// Remove mocks base method.
func (m *MockFileSystemInterface) Remove(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockFileSystemInterfaceMockRecorder) Remove(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockFileSystemInterface)(nil).Remove), name)
}

// Stat mocks base method.
func (m *MockFileSystemInterface) Stat(name string) (os.FileInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stat", name)
	ret0, _ := ret[0].(os.FileInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stat indicates an expected call of Stat.
func (mr *MockFileSystemInterfaceMockRecorder) Stat(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stat", reflect.TypeOf((*MockFileSystemInterface)(nil).Stat), name)
}

// UserHomeDir mocks base method.
func (m *MockFileSystemInterface) UserHomeDir() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UserHomeDir")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UserHomeDir indicates an expected call of UserHomeDir.
func (mr *MockFileSystemInterfaceMockRecorder) UserHomeDir() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UserHomeDir", reflect.TypeOf((*MockFileSystemInterface)(nil).UserHomeDir))
}

// WriteFile mocks base method.
func (m *MockFileSystemInterface) WriteFile(name string, data []byte, perm os.FileMode) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteFile", name, data, perm)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteFile indicates an expected call of WriteFile.
func (mr *MockFileSystemInterfaceMockRecorder) WriteFile(name, data, perm interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteFile", reflect.TypeOf((*MockFileSystemInterface)(nil).WriteFile), name, data, perm)
}

// MockCommandExecutor is a mock of CommandExecutor interface.
type MockCommandExecutor struct {
	ctrl     *gomock.Controller
	recorder *MockCommandExecutorMockRecorder
}

// MockCommandExecutorMockRecorder is the mock recorder for MockCommandExecutor.
type MockCommandExecutorMockRecorder struct {
	mock *MockCommandExecutor
}

// NewMockCommandExecutor creates a new mock instance.
func NewMockCommandExecutor(ctrl *gomock.Controller) *MockCommandExecutor {
	mock := &MockCommandExecutor{ctrl: ctrl}
	mock.recorder = &MockCommandExecutorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCommandExecutor) EXPECT() *MockCommandExecutorMockRecorder {
	return m.recorder
}

// LookPath mocks base method.
func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookPath", file)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LookPath indicates an expected call of LookPath.
func (mr *MockCommandExecutorMockRecorder) LookPath(file interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookPath", reflect.TypeOf((*MockCommandExecutor)(nil).LookPath), file)
}

// RunCommand mocks base method.
func (m *MockCommandExecutor) RunCommand(name string, args ...string) ([]byte, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{name}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RunCommand", varargs...)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunCommand indicates an expected call of RunCommand.
func (mr *MockCommandExecutorMockRecorder) RunCommand(name interface{}, args ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{name}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunCommand", reflect.TypeOf((*MockCommandExecutor)(nil).RunCommand), varargs...)
}

// RunCommandWithInput mocks base method.
func (m *MockCommandExecutor) RunCommandWithInput(name, input string, args ...string) ([]byte, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{name, input}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RunCommandWithInput", varargs...)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunCommandWithInput indicates an expected call of RunCommandWithInput.
func (mr *MockCommandExecutorMockRecorder) RunCommandWithInput(name, input interface{}, args ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{name, input}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunCommandWithInput", reflect.TypeOf((*MockCommandExecutor)(nil).RunCommandWithInput), varargs...)
}

// RunInteractiveCommand mocks base method.
func (m *MockCommandExecutor) RunInteractiveCommand(ctx context.Context, name string, args ...string) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, name}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "RunInteractiveCommand", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// RunInteractiveCommand indicates an expected call of RunInteractiveCommand.
func (mr *MockCommandExecutorMockRecorder) RunInteractiveCommand(ctx, name interface{}, args ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, name}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunInteractiveCommand", reflect.TypeOf((*MockCommandExecutor)(nil).RunInteractiveCommand), varargs...)
}

// MockSSHExecutorInterface is a mock of SSHExecutorInterface interface.
type MockSSHExecutorInterface struct {
	ctrl     *gomock.Controller
	recorder *MockSSHExecutorInterfaceMockRecorder
}

// MockSSHExecutorInterfaceMockRecorder is the mock recorder for MockSSHExecutorInterface.
type MockSSHExecutorInterfaceMockRecorder struct {
	mock *MockSSHExecutorInterface
}

// NewMockSSHExecutorInterface creates a new mock instance.
func NewMockSSHExecutorInterface(ctrl *gomock.Controller) *MockSSHExecutorInterface {
	mock := &MockSSHExecutorInterface{ctrl: ctrl}
	mock.recorder = &MockSSHExecutorInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSSHExecutorInterface) EXPECT() *MockSSHExecutorInterfaceMockRecorder {
	return m.recorder
}

// Execute mocks base method.
func (m *MockSSHExecutorInterface) Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", args, stdin, stdout, stderr)
	ret0, _ := ret[0].(error)
	return ret0
}

// Execute indicates an expected call of Execute.
func (mr *MockSSHExecutorInterfaceMockRecorder) Execute(args, stdin, stdout, stderr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockSSHExecutorInterface)(nil).Execute), args, stdin, stdout, stderr)
}

// MockOSDetector is a mock of OSDetector interface.
type MockOSDetector struct {
	ctrl     *gomock.Controller
	recorder *MockOSDetectorMockRecorder
}

// MockOSDetectorMockRecorder is the mock recorder for MockOSDetector.
type MockOSDetectorMockRecorder struct {
	mock *MockOSDetector
}

// NewMockOSDetector creates a new mock instance.
func NewMockOSDetector(ctrl *gomock.Controller) *MockOSDetector {
	mock := &MockOSDetector{ctrl: ctrl}
	mock.recorder = &MockOSDetectorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOSDetector) EXPECT() *MockOSDetectorMockRecorder {
	return m.recorder
}

// GetOS mocks base method.
func (m *MockOSDetector) GetOS() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOS")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetOS indicates an expected call of GetOS.
func (mr *MockOSDetectorMockRecorder) GetOS() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOS", reflect.TypeOf((*MockOSDetector)(nil).GetOS))
}
