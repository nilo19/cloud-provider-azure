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

// Code generated by client-gen. DO NOT EDIT.
package virtualmachinescalesetvmclient

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/tracing"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"sigs.k8s.io/cloud-provider-azure/pkg/azclient/utils"
)

type Client struct {
	*armcompute.VirtualMachineScaleSetVMsClient
	subscriptionID string
	tracer         tracing.Tracer
}

func New(subscriptionID string, credential azcore.TokenCredential, options *arm.ClientOptions) (Interface, error) {
	if options == nil {
		options = utils.GetDefaultOption()
	}
	tr := options.TracingProvider.NewTracer(utils.ModuleName, utils.ModuleVersion)

	client, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, credential, options)
	if err != nil {
		return nil, err
	}
	return &Client{
		VirtualMachineScaleSetVMsClient: client,
		subscriptionID:                  subscriptionID,
		tracer:                          tr,
	}, nil
}

const GetOperationName = "VirtualMachineScaleSetVMsClient.Get"

// Get gets the VirtualMachineScaleSetVM
func (client *Client) Get(ctx context.Context, resourceGroupName string, parentResourceName string, resourceName string) (result *armcompute.VirtualMachineScaleSetVM, rerr error) {

	ctx = utils.ContextWithClientName(ctx, "VirtualMachineScaleSetVMsClient")
	ctx = utils.ContextWithRequestMethod(ctx, "Get")
	ctx = utils.ContextWithResourceGroupName(ctx, resourceGroupName)
	ctx = utils.ContextWithSubscriptionID(ctx, client.subscriptionID)
	ctx, endSpan := runtime.StartSpan(ctx, GetOperationName, client.tracer, nil)
	defer endSpan(rerr)
	resp, err := client.VirtualMachineScaleSetVMsClient.Get(ctx, resourceGroupName, parentResourceName, resourceName, nil)
	if err != nil {
		return nil, err
	}
	//handle statuscode
	return &resp.VirtualMachineScaleSetVM, nil
}

const DeleteOperationName = "VirtualMachineScaleSetVMsClient.Delete"

// Delete deletes a VirtualMachineScaleSetVM by name.
func (client *Client) Delete(ctx context.Context, resourceGroupName string, parentResourceName string, resourceName string) (err error) {
	ctx = utils.ContextWithClientName(ctx, "VirtualMachineScaleSetVMsClient")
	ctx = utils.ContextWithRequestMethod(ctx, "Delete")
	ctx = utils.ContextWithResourceGroupName(ctx, resourceGroupName)
	ctx = utils.ContextWithSubscriptionID(ctx, client.subscriptionID)
	ctx, endSpan := runtime.StartSpan(ctx, DeleteOperationName, client.tracer, nil)
	defer endSpan(err)
	_, err = utils.NewPollerWrapper(client.BeginDelete(ctx, resourceGroupName, parentResourceName, resourceName, nil)).WaitforPollerResp(ctx)
	return err
}