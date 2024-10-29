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
	"encoding/base64"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ks-tool/ks/pkg/common"
	"github.com/ks-tool/ks/pkg/utils"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1/instancegroup"
	"github.com/yandex-cloud/go-sdk/operation"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
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

type ComputeInstanceConfig struct {
	Name       string `mapstructure:"name"`
	PlatformID string `mapstructure:"platform-id"`

	FolderID string `mapstructure:"folder-id"`
	SubnetID string `mapstructure:"subnet-id"`
	Zone     string `mapstructure:"zone"`
	Address  string `mapstructure:"address"`

	Cores        uint `mapstructure:"cores"`
	CoreFraction uint `mapstructure:"core-fraction"`
	Memory       uint `mapstructure:"memory"`

	DiskType string `mapstructure:"disk-type"`
	DiskID   string `mapstructure:"disk-id"`
	DiskSize uint   `mapstructure:"disk-size"`

	Preemptible    bool   `mapstructure:"preemptible"`
	NoPublicIP     bool   `mapstructure:"no-public-ip"`
	ServiceAccount string `mapstructure:"sa"`

	UserDataFile      string   `mapstructure:"user-data-file"`
	User              string   `mapstructure:"user"`
	SshPublicKeyFiles []string `mapstructure:"ssh-pub"`
	Shell             string   `mapstructure:"shell"`

	ClusterID         string
	Metadata          map[string]string
	Labels            map[string]string
	SshAuthorizedKeys []string
}

func (cfg *ComputeInstanceConfig) SetUserData(tpl string) error {
	if err := cfg.fillSshKeys(); err != nil {
		return err
	}

	if len(tpl) == 0 {
		tpl = common.UserDataTemplate
	}
	if cfg.Metadata == nil {
		cfg.Metadata = make(map[string]string)
	}
	if _, ok := cfg.Metadata[common.UserDataKey]; !ok {
		userData, err := utils.Template(tpl, map[string]any{
			"user":              cfg.User,
			"sshAuthorizedKeys": cfg.SshAuthorizedKeys,
			"shell":             cfg.Shell,
		})
		if err != nil {
			return err
		}

		cfg.Metadata[common.UserDataKey] = userData
	}

	return nil
}

func (cfg *ComputeInstanceConfig) fillSshKeys() error {
	if len(cfg.SshAuthorizedKeys) > 0 {
		return nil
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}

	if len(cfg.SshPublicKeyFiles) > 0 {
		for _, key := range cfg.SshPublicKeyFiles {
			key, err = homedir.Expand(key)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(key)
			if err != nil {
				return err
			}

			keyData := strings.TrimSpace(string(b))
			keyParts := strings.Split(keyData, " ")
			cfg.SshAuthorizedKeys = append(cfg.SshAuthorizedKeys, strings.Join(keyParts[:2], " "))
		}

		return nil
	}

	sshDir := filepath.Join(usr.HomeDir, ".ssh")
	dir, err := os.ReadDir(sshDir)
	if err != nil {
		return err
	}

	for _, file := range dir {
		filename := file.Name()
		if file.IsDir() || filename == "config" || strings.HasPrefix(filename, "known_hosts") {
			continue
		}

		b, err := os.ReadFile(filepath.Join(sshDir, filename))
		if err != nil {
			return err
		}

		keyData := strings.TrimSpace(string(b))
		if strings.Contains(keyData, "PRIVATE KEY") {
			continue
		}

		keyParts := strings.Split(keyData, " ")
		if len(keyParts) < 2 {
			continue
		}

		keyBytes, err := base64.StdEncoding.DecodeString(keyParts[1])
		if err != nil {
			continue
		}

		if _, err := ssh.ParsePublicKey(keyBytes); err == nil {
			key := strings.Join(keyParts[:2], " ")
			cfg.SshAuthorizedKeys = append(cfg.SshAuthorizedKeys, key)
		}
	}

	return nil
}

func (c *Client) ComputeInstanceCreate(ctx context.Context, cfg *ComputeInstanceConfig) (*operation.Operation, error) {
	if len(cfg.Zone) == 0 {
		cfg.Zone = DefaultZone
	}
	if len(cfg.PlatformID) == 0 {
		cfg.PlatformID = DefaultPlatformID
	}

	computeResources := &compute.ResourcesSpec{
		Cores:        int64(cfg.Cores),
		Memory:       utils.ToGib(cfg.Memory),
		CoreFraction: int64(cfg.CoreFraction),
	}
	if computeResources.Cores == 0 {
		computeResources.Cores = DefaultCores
	}
	if computeResources.Memory == 0 {
		computeResources.Memory = DefaultMemoryGib * utils.Gib
	}
	if computeResources.CoreFraction == 0 {
		computeResources.CoreFraction = DefaultCoreFraction
	}

	diskSpec := &compute.AttachedDiskSpec_DiskSpec{
		TypeId: cfg.DiskType,
		Size:   utils.ToGib(cfg.DiskSize),
		Source: &compute.AttachedDiskSpec_DiskSpec_ImageId{
			ImageId: cfg.DiskID,
		},
	}
	if len(diskSpec.TypeId) == 0 {
		diskSpec.TypeId = DefaultDiskType
	}
	if diskSpec.Size == 0 {
		diskSpec.Size = DefaultDiskSizeGib * utils.Gib
	}
	if len(diskSpec.GetImageId()) == 0 {
		diskSpec.SetImageId(DefaultDiskID)
	}

	networkSpec := &compute.NetworkInterfaceSpec{
		SubnetId:             cfg.SubnetID,
		PrimaryV4AddressSpec: &compute.PrimaryAddressSpec{},
	}
	if len(networkSpec.SubnetId) == 0 {
		subnetId, err := c.FirstSubnetInZone(ctx, cfg.FolderID, cfg.Zone)
		if err != nil {
			return nil, err
		}

		networkSpec.SubnetId = subnetId
	}
	if !cfg.NoPublicIP {
		networkSpec.PrimaryV4AddressSpec.OneToOneNatSpec = &compute.OneToOneNatSpec{
			IpVersion: compute.IpVersion_IPV4,
			Address:   cfg.Address,
		}
	}

	request := &compute.CreateInstanceRequest{
		FolderId:      cfg.FolderID,
		Name:          cfg.Name,
		Labels:        cfg.Labels,
		ZoneId:        cfg.Zone,
		PlatformId:    cfg.PlatformID,
		ResourcesSpec: computeResources,
		MetadataOptions: &compute.MetadataOptions{
			GceHttpEndpoint: 1,
			GceHttpToken:    1,
		},
		Metadata: cfg.Metadata,
		BootDiskSpec: &compute.AttachedDiskSpec{
			AutoDelete: true,
			Disk: &compute.AttachedDiskSpec_DiskSpec_{
				DiskSpec: diskSpec,
			},
		},
		NetworkInterfaceSpecs: []*compute.NetworkInterfaceSpec{networkSpec},
		SchedulingPolicy:      &compute.SchedulingPolicy{Preemptible: cfg.Preemptible},
	}

	if len(cfg.ServiceAccount) > 0 {
		var err error
		request.ServiceAccountId, err = c.IAMServiceAccountGetIdByName(ctx, cfg.ServiceAccount)
		if err != nil {
			return nil, err
		}
	}

	return c.sdk.WrapOperation(c.sdk.Compute().Instance().Create(ctx, request))
}

func (c *Client) ComputeInstanceDelete(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &compute.DeleteInstanceRequest{InstanceId: id}
	return c.sdk.WrapOperation(c.sdk.Compute().Instance().Delete(cctx, op))
}

func (c *Client) ComputeInstanceList(
	ctx context.Context,
	folderID string,
	lbl map[string]string,
	filters ...Filter,
) ([]*compute.Instance, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &compute.ListInstancesRequest{
		FolderId: folderID,
		PageSize: 1000,
	}

	var filter []string
	for _, f := range filters {
		filter = append(filter, f.String())
	}
	op.Filter = strings.Join(filter, " AND ")

	lst, err := c.sdk.Compute().Instance().List(cctx, op)
	if err != nil {
		return nil, err
	}

	var out []*compute.Instance
	for _, item := range lst.Instances {
		if utils.AllInMap(item.Labels, lbl) {
			out = append(out, item)
		}
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

func (c *Client) ComputeInstanceGroupCreate(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.CreateInstanceGroupRequest{}
	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Create(cctx, op))
}

func (c *Client) ComputeInstanceGroupDelete(ctx context.Context, id string) (*operation.Operation, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	op := &instancegroup.DeleteInstanceGroupRequest{InstanceGroupId: id}
	return c.sdk.WrapOperation(c.sdk.InstanceGroup().InstanceGroup().Delete(cctx, op))
}
