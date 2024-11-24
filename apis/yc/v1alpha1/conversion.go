/*
 Copyright (c) 2024 Alexey Shulutkov <github@shulutkov.ru>

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

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ks-tool/ks/apis/scopes"
	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/utils"

	computev1 "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1/instancegroup"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/sdkresolvers"

	"k8s.io/apimachinery/pkg/conversion"
)

var (
	folderResolver = sdkresolvers.FolderResolver
	imageResolver  = sdkresolvers.ImageResolver
	saResolver     = sdkresolvers.ServiceAccountResolver
	sgResolver     = sdkresolvers.SecurityGroupResolver
	subnetResolver = sdkresolvers.SubnetResolver
)

type resolverFunc func(name string, opts ...sdkresolvers.ResolveOption) ycsdk.Resolver

func resolver(fn resolverFunc, sdk *ycsdk.SDK, name string, opts ...sdkresolvers.ResolveOption) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := fn(name, opts...)
	if err := res.Run(ctx, sdk); err != nil {
		return "", err
	}

	return res.ID(), res.Err()
}

func resolvers(fn resolverFunc, sdk *ycsdk.SDK, names []string, opts ...sdkresolvers.ResolveOption) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := make([]ycsdk.Resolver, len(names))
	for i, name := range names {
		res[i] = fn(name, opts...)
	}

	if err := sdk.Resolve(ctx, res...); err != nil {
		return nil, err
	}

	var ids []string
	var err *common.GroupedErrors
	for _, r := range res {
		if r.Err() != nil {
			if err == nil {
				err = new(common.GroupedErrors)
			}
			err.Group = append(err.Group, r.Err())
		} else {
			ids = append(ids, r.ID())
		}
	}

	return ids, err
}

func Convert_v1alpha1_ComputeInstance_To_v1_CreateInstanceRequest(in *ComputeInstance, out *computev1.CreateInstanceRequest, s conversion.Scope) error {
	spec := in.Spec

	computeResources := &computev1.ResourcesSpec{
		Cores:        int64(spec.Resources.Cpu),
		Memory:       utils.ToGib(spec.Resources.Memory),
		CoreFraction: int64(spec.Resources.CoreFraction),
	}

	scope := s.Meta().Context.(scopes.CreateInstanceRequestConversionScope)

	folderId := scope.FolderId
	if len(spec.Folder) > 0 {
		var err error
		folderId, err = resolver(folderResolver, scope.Sdk, spec.Folder, sdkresolvers.CloudID(scope.CloudId))
		if err != nil {
			return err
		}
	}

	resolveOptFolderId := sdkresolvers.FolderID(folderId)

	imageId, err := resolver(imageResolver, scope.Sdk, spec.Disk.Image, sdkresolvers.FolderID("standard-images"))
	var e *sdkresolvers.ErrNotFound
	if errors.As(err, &e) {
		imageId, err = resolver(imageResolver, scope.Sdk, spec.Disk.Image, resolveOptFolderId)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	diskSpec := &computev1.AttachedDiskSpec_DiskSpec{
		TypeId: spec.Disk.Type,
		Size:   utils.ToGib(spec.Disk.Size),
		Source: &computev1.AttachedDiskSpec_DiskSpec_ImageId{
			ImageId: imageId,
		},
	}

	var networks []*computev1.NetworkInterfaceSpec
	for _, netif := range spec.NetworkInterfaces {
		var sg []string
		if len(netif.SecurityGroups) > 0 {
			sg, err = resolvers(sgResolver, scope.Sdk, netif.SecurityGroups, resolveOptFolderId)
		}

		subnetId, err := resolver(subnetResolver, scope.Sdk, netif.Subnet, resolveOptFolderId)
		if err != nil {
			return err
		}

		networkSpec := &computev1.NetworkInterfaceSpec{
			SubnetId:             subnetId,
			PrimaryV4AddressSpec: &computev1.PrimaryAddressSpec{Address: netif.PrivateIp},
			SecurityGroupIds:     sg,
		}
		if netif.PublicIp != nil {
			networkSpec.PrimaryV4AddressSpec.OneToOneNatSpec = &computev1.OneToOneNatSpec{
				IpVersion: computev1.IpVersion_IPV4,
				Address:   *netif.PublicIp,
			}
		}

		networks = append(networks, networkSpec)
	}

	if len(networks) == 0 {
		if err = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := scope.Sdk.VPC().Subnet().List(ctx, &vpc.ListSubnetsRequest{FolderId: folderId})
			if err != nil {
				return err
			}

			for _, subnet := range resp.Subnets {
				if subnet.ZoneId == spec.Zone {
					networks = append(networks, &computev1.NetworkInterfaceSpec{
						SubnetId: subnet.Id,
						PrimaryV4AddressSpec: &computev1.PrimaryAddressSpec{
							OneToOneNatSpec: &computev1.OneToOneNatSpec{
								IpVersion: computev1.IpVersion_IPV4,
							}},
					})

					return nil
				}
			}

			return fmt.Errorf("no subnet found in zone %q", spec.Zone)
		}(); err != nil {
			return err
		}
	}

	metadata := in.Annotations
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata[common.UserDataKey] = spec.UserData

	metaOptions := &computev1.MetadataOptions{
		GceHttpEndpoint: 1,
	}

	var saId string
	if len(spec.ServiceAccount) > 0 {
		saId, err = resolver(saResolver, scope.Sdk, spec.ServiceAccount, resolveOptFolderId)
		if err != nil {
			return err
		}
		metaOptions.GceHttpToken = 1
	}

	*out = computev1.CreateInstanceRequest{
		FolderId:         folderId,
		Name:             in.Name,
		Labels:           in.Labels,
		ZoneId:           spec.Zone,
		PlatformId:       spec.Platform,
		ResourcesSpec:    computeResources,
		MetadataOptions:  metaOptions,
		Metadata:         metadata,
		ServiceAccountId: saId,
		BootDiskSpec: &computev1.AttachedDiskSpec{
			Mode:       computev1.AttachedDiskSpec_READ_WRITE,
			AutoDelete: true,
			Disk: &computev1.AttachedDiskSpec_DiskSpec_{
				DiskSpec: diskSpec,
			},
		},
		NetworkInterfaceSpecs: networks,
		SchedulingPolicy:      &computev1.SchedulingPolicy{Preemptible: spec.Preemptible},
	}

	return nil
}

func Convert_v1alpha1_ComputeInstance_To_instancegroup_CreateInstanceGroupRequest(in *ComputeInstance, out *instancegroup.CreateInstanceGroupRequest, s conversion.Scope) error {
	cir := new(computev1.CreateInstanceRequest)
	if err := Convert_v1alpha1_ComputeInstance_To_v1_CreateInstanceRequest(in, cir, s); err != nil {
		return err
	}

	tpl := new(instancegroup.InstanceTemplate)
	tpl.Labels = cir.Labels
	tpl.PlatformId = cir.PlatformId
	tpl.ResourcesSpec = &instancegroup.ResourcesSpec{
		Memory:       cir.ResourcesSpec.Memory,
		Cores:        cir.ResourcesSpec.Cores,
		CoreFraction: cir.ResourcesSpec.CoreFraction,
		Gpus:         cir.ResourcesSpec.Gpus,
	}
	tpl.Metadata = cir.Metadata

	diskSpec := cir.BootDiskSpec.Disk.(*computev1.AttachedDiskSpec_DiskSpec_).DiskSpec
	srcDisk := diskSpec.Source.(*computev1.AttachedDiskSpec_DiskSpec_ImageId)
	tpl.BootDiskSpec = &instancegroup.AttachedDiskSpec{
		Mode: instancegroup.AttachedDiskSpec_Mode(cir.BootDiskSpec.Mode),
		DiskSpec: &instancegroup.AttachedDiskSpec_DiskSpec{
			TypeId:                      diskSpec.TypeId,
			Size:                        diskSpec.Size,
			SourceOneof:                 &instancegroup.AttachedDiskSpec_DiskSpec_ImageId{ImageId: srcDisk.ImageId},
			PreserveAfterInstanceDelete: false,
		},
	}

	tpl.NetworkInterfaceSpecs = make([]*instancegroup.NetworkInterfaceSpec, len(cir.NetworkInterfaceSpecs))
	for i, netif := range cir.NetworkInterfaceSpecs {
		ipv4Spec := &instancegroup.PrimaryAddressSpec{
			Address: netif.PrimaryV4AddressSpec.Address,
		}
		if netif.PrimaryV4AddressSpec.OneToOneNatSpec != nil {
			ipv4Spec.OneToOneNatSpec = &instancegroup.OneToOneNatSpec{
				IpVersion: instancegroup.IpVersion(netif.PrimaryV4AddressSpec.OneToOneNatSpec.IpVersion),
				Address:   netif.PrimaryV4AddressSpec.OneToOneNatSpec.Address,
			}
		}
		tpl.NetworkInterfaceSpecs[i] = &instancegroup.NetworkInterfaceSpec{
			SubnetIds:            []string{netif.SubnetId},
			PrimaryV4AddressSpec: ipv4Spec,
			SecurityGroupIds:     netif.SecurityGroupIds,
		}
	}
	tpl.SchedulingPolicy = &instancegroup.SchedulingPolicy{Preemptible: cir.SchedulingPolicy.Preemptible}
	tpl.MetadataOptions = &instancegroup.MetadataOptions{
		GceHttpEndpoint:   instancegroup.MetadataOption(cir.MetadataOptions.GceHttpEndpoint),
		AwsV1HttpEndpoint: instancegroup.MetadataOption(cir.MetadataOptions.AwsV1HttpEndpoint),
		GceHttpToken:      instancegroup.MetadataOption(cir.MetadataOptions.GceHttpToken),
		AwsV1HttpToken:    instancegroup.MetadataOption(cir.MetadataOptions.AwsV1HttpToken),
	}

	out.FolderId = cir.FolderId
	out.Name = cir.Name
	out.Description = cir.Description
	out.Labels = cir.Labels
	out.ServiceAccountId = cir.ServiceAccountId
	out.InstanceTemplate = tpl
	out.ScalePolicy = &instancegroup.ScalePolicy{
		ScaleType: &instancegroup.ScalePolicy_FixedScale_{
			FixedScale: &instancegroup.ScalePolicy_FixedScale{},
		},
	}
	out.DeployPolicy = &instancegroup.DeployPolicy{
		MaxExpansion: 1,
	}
	out.AllocationPolicy = &instancegroup.AllocationPolicy{
		Zones: []*instancegroup.AllocationPolicy_Zone{
			{
				ZoneId: cir.ZoneId,
			},
		},
	}

	return nil
}

func Convert_v1alpha1_Kubernetes_To_v1alpha1_KubernetesRequest(in *Kubernetes, out *KubernetesRequest, s conversion.Scope) error {
	controlPlain := &ComputeInstance{
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec.ControlPlain,
	}

	if out.ControlPlain == nil {
		out.ControlPlain = &computev1.CreateInstanceRequest{}
	}

	if err := Convert_v1alpha1_ComputeInstance_To_v1_CreateInstanceRequest(controlPlain, out.ControlPlain, s); err != nil {
		return err
	}

	out.NodeGroups = make([]*instancegroup.CreateInstanceGroupRequest, len(in.Spec.NodeGroups))
	for i, ng := range in.Spec.NodeGroups {
		cigr := new(instancegroup.CreateInstanceGroupRequest)
		ci := &ComputeInstance{
			ObjectMeta: in.ObjectMeta,
			Spec:       ng,
		}
		if err := Convert_v1alpha1_ComputeInstance_To_instancegroup_CreateInstanceGroupRequest(ci, cigr, s); err != nil {
			return err
		}
		out.NodeGroups[i] = cigr
	}

	return nil
}
