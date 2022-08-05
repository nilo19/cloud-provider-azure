// /*
// Copyright The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
//

// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/azureclients/blobclient/interface.go

// Package mockblobclient is a generated GoMock package.
package mockblobclient

import (
	context "context"
	reflect "reflect"

	storage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2021-09-01/storage"
	gomock "github.com/golang/mock/gomock"
)

// MockInterface is a mock of Interface interface.
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface.
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance.
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// CreateContainer mocks base method.
func (m *MockInterface) CreateContainer(ctx context.Context, resourceGroupName, accountName, containerName string, blobContainer storage.BlobContainer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateContainer", ctx, resourceGroupName, accountName, containerName, blobContainer)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateContainer indicates an expected call of CreateContainer.
func (mr *MockInterfaceMockRecorder) CreateContainer(ctx, resourceGroupName, accountName, containerName, blobContainer interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateContainer", reflect.TypeOf((*MockInterface)(nil).CreateContainer), ctx, resourceGroupName, accountName, containerName, blobContainer)
}

// DeleteContainer mocks base method.
func (m *MockInterface) DeleteContainer(ctx context.Context, resourceGroupName, accountName, containerName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteContainer", ctx, resourceGroupName, accountName, containerName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteContainer indicates an expected call of DeleteContainer.
func (mr *MockInterfaceMockRecorder) DeleteContainer(ctx, resourceGroupName, accountName, containerName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteContainer", reflect.TypeOf((*MockInterface)(nil).DeleteContainer), ctx, resourceGroupName, accountName, containerName)
}

// GetContainer mocks base method.
func (m *MockInterface) GetContainer(ctx context.Context, resourceGroupName, accountName, containerName string) (storage.BlobContainer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetContainer", ctx, resourceGroupName, accountName, containerName)
	ret0, _ := ret[0].(storage.BlobContainer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetContainer indicates an expected call of GetContainer.
func (mr *MockInterfaceMockRecorder) GetContainer(ctx, resourceGroupName, accountName, containerName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetContainer", reflect.TypeOf((*MockInterface)(nil).GetContainer), ctx, resourceGroupName, accountName, containerName)
}