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
package sshpublickeyresourceclient

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var newResource *armcompute.SSHPublicKeyResource

func init() {
	additionalTestCases = func() {
		When("create requests are raised", func() {
			It("should not return error", func(ctx context.Context) {
				newResource, err := realClient.Create(ctx, resourceGroupName, resourceName, *newResource)
				Expect(err).NotTo(HaveOccurred())
				Expect(newResource).NotTo(BeNil())
			})
		})
		When("generate requests are raised", func() {
			It("should not return error", func(ctx context.Context) {
				newResource, err := realClient.GenerateKeyPair(ctx, resourceGroupName, resourceName)
				Expect(err).NotTo(HaveOccurred())
				Expect(newResource).NotTo(BeNil())
			})
		})
	}

	beforeAllFunc = func(ctx context.Context) {
		newResource = &armcompute.SSHPublicKeyResource{
			Location: to.Ptr(location),
		}
	}
	afterAllFunc = func(ctx context.Context) {
		err := realClient.Delete(ctx, resourceGroupName, resourceName)
		Expect(err).NotTo(HaveOccurred())
	}
}
