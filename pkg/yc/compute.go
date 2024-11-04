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
	"github.com/yandex-cloud/go-sdk/sdkresolvers"

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
	PlatformID string `mapstructure:"platform"`

	FolderID string `mapstructure:"folder-id"`
	SubnetID string `mapstructure:"subnet-id"`
	Zone     string `mapstructure:"zone"`
	Address  string `mapstructure:"address"`

	Cores        uint `mapstructure:"cores"`
	CoreFraction uint `mapstructure:"core-fraction"`
	Memory       uint `mapstructure:"memory"`

	DiskType string `mapstructure:"disk-type"`
	ImageID  string `mapstructure:"image-id"`
	DiskSize uint   `mapstructure:"disk-size"`

	Preemptible    bool   `mapstructure:"preemptible"`
	NoPublicIP     bool   `mapstructure:"no-public-ip"`
	ServiceAccount string `mapstructure:"sa"`

	UserDataFile      string   `mapstructure:"user-data-file"`
	User              string   `mapstructure:"user"`
	SshPublicKeyFiles []string `mapstructure:"ssh-key"`
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

func (c *Client) ComputeInstanceGroupCreate(ctx context.Context, name string) (*operation.Operation, error) {
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
