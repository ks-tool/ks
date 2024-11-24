/*
Copyright © 2024 Alexey Shulutkov <github@shulutkov.ru>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package yc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ks-tool/ks/pkg/utils"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1/instancegroup"
	"github.com/yandex-cloud/go-sdk/operation"
	"github.com/yandex-cloud/go-sdk/sdkresolvers"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var (
	statuses = map[string]struct{}{
		"Crashed":      {},
		"Deleting":     {},
		"Error":        {},
		"Provisioning": {},
		"Restarting":   {},
		"Running":      {},
		"Starting":     {},
		"Stopped":      {},
		"Stopping":     {},
		"Updating":     {},
	}

	Statuses = func() string {
		var out []string
		for k := range statuses {
			out = append(out, k)
		}

		sort.Strings(out)
		return strings.Join(out, ", ")
	}

	AllowStatus = func(s string) bool {
		s = strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		_, ok := statuses[s]
		return ok
	}
)

type FilterOperator string

const (
	OperatorEq    FilterOperator = "="
	OperatorNe    FilterOperator = "!="
	OperatorIn    FilterOperator = "IN"
	OperatorNotIn FilterOperator = "NOT IN"
)

type Filter struct {
	Field    string
	Operator FilterOperator
	Value    any
}

func (f Filter) String() string {
	return fmt.Sprintf(`%s %s %q`, f.Field, f.Operator, f.Value)
}

func (c *Client) ComputeInstanceCreate(ctx context.Context, req *compute.CreateInstanceRequest) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	return c.sdk.WrapOperation(c.sdk.Compute().Instance().Create(cctx, req))
}

func (c *Client) ComputeInstanceDelete(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &compute.DeleteInstanceRequest{InstanceId: id}
	return c.sdk.WrapOperation(c.sdk.Compute().Instance().Delete(cctx, op))
}

func (c *Client) ComputeInstanceList(
	ctx context.Context,
	folderId string,
	lbl map[string]string,
	filters ...Filter,
) ([]*compute.Instance, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if len(folderId) == 0 {
		folderId = c.folderId
	}

	op := &compute.ListInstancesRequest{
		FolderId: folderId,
	}

	var filter []string
	for _, f := range filters {
		filter = append(filter, f.String())
	}
	op.Filter = strings.Join(filter, " AND ")

	var out []*compute.Instance
	iter := c.sdk.Compute().Instance().InstanceIterator(cctx, op)
	for iter.Next() {
		item := iter.Value()
		if utils.AllInMap(item.Labels, lbl) {
			out = append(out, item)
		}
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) ComputeInstanceStart(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &compute.StartInstanceRequest{InstanceId: id}
	return c.sdk.WrapOperation(c.sdk.Compute().Instance().Start(cctx, op))
}

func (c *Client) ComputeInstanceStop(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &compute.StopInstanceRequest{InstanceId: id}
	return c.sdk.WrapOperation(c.sdk.Compute().Instance().Stop(cctx, op))
}

func (c *Client) InstanceGroupCreate(ctx context.Context, req *instancegroup.CreateInstanceGroupRequest) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Create(cctx, req))
}

func (c *Client) InstanceGroupDelete(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.DeleteInstanceGroupRequest{InstanceGroupId: id}
	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Delete(cctx, op))
}

func (c *Client) InstanceGroupStart(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.StartInstanceGroupRequest{InstanceGroupId: id}
	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Start(cctx, op))
}

func (c *Client) InstanceGroupStop(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.StopInstanceGroupRequest{InstanceGroupId: id}
	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Stop(cctx, op))
}

func (c *Client) InstanceGroupScale(ctx context.Context, id string, n int) (*operation.Operation, error) {
	if n < 0 {
		n = 0
	}

	op := &instancegroup.UpdateInstanceGroupRequest{
		InstanceGroupId: id,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"scale_policy.fixed_scale.size"},
		},
		ScalePolicy: &instancegroup.ScalePolicy{
			ScaleType: &instancegroup.ScalePolicy_FixedScale_{
				FixedScale: &instancegroup.ScalePolicy_FixedScale{
					Size: int64(n)}}}}

	return c.InstanceGroupUpdate(ctx, op)
}

func (c *Client) InstanceGroupInstancesDelete(ctx context.Context, id string, managedInstanceIds ...string) (*operation.Operation, error) {
	if len(managedInstanceIds) == 0 {
		return nil, errors.New("managed instance ids is empty")
	}

	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.DeleteInstancesRequest{InstanceGroupId: id, ManagedInstanceIds: managedInstanceIds}
	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().DeleteInstances(cctx, op))
}

func (c *Client) InstanceGroupList(ctx context.Context, folderId string, lbl map[string]string) ([]*instancegroup.InstanceGroup, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if len(folderId) == 0 {
		folderId = c.folderId
	}

	op := &instancegroup.ListInstanceGroupsRequest{FolderId: folderId}
	iter := c.sdk.InstanceGroup().InstanceGroup().InstanceGroupIterator(cctx, op)

	var groups []*instancegroup.InstanceGroup
	for iter.Next() {
		item := iter.Value()
		if utils.AllInMap(item.Labels, lbl) {
			groups = append(groups, item)
		}
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return groups, nil
}

func (c *Client) InstanceGroupInstancesList(ctx context.Context, id string) ([]*instancegroup.ManagedInstance, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.ListInstanceGroupInstancesRequest{InstanceGroupId: id}
	iter := c.sdk.InstanceGroup().InstanceGroup().InstanceGroupInstancesIterator(cctx, op)

	var lst []*instancegroup.ManagedInstance
	for iter.Next() {
		lst = append(lst, iter.Value())
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return lst, nil
}

func (c *Client) InstanceGroupUpdate(ctx context.Context, req *instancegroup.UpdateInstanceGroupRequest) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Update(cctx, req))
}

func (c *Client) ComputeImageGetId(ctx context.Context, name, folderId string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	if len(folderId) == 0 {
		folderId = c.folderId
	}

	iter := c.sdk.Compute().Image().ImageIterator(cctx, &compute.ListImagesRequest{FolderId: folderId})

	for iter.Next() {
		image := iter.Value()
		if image.Name == name {
			return image.Id, nil
		}
	}

	if err := iter.Error(); err != nil {
		return "", err
	}

	return "", sdkresolvers.NewErrNotFound(fmt.Sprintf("image %q not found", name))
}
