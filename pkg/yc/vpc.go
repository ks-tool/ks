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
	"fmt"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	"github.com/yandex-cloud/go-sdk/sdkresolvers"
)

func (c *Client) FirstSubnetInZone(ctx context.Context, folderID string, zone string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.sdk.VPC().Subnet().List(cctx, &vpc.ListSubnetsRequest{
		FolderId: folderID,
		PageSize: sdkresolvers.DefaultResolverPageSize,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Subnets) == 0 {
		return "", fmt.Errorf("no subnet in zone %q", zone)
	}

	subnetID := ""
	for _, subnet := range resp.Subnets {
		if subnet.ZoneId != zone {
			continue
		}
		subnetID = subnet.Id
		break
	}
	if subnetID == "" {
		return "", fmt.Errorf("subnet not found")
	}

	return subnetID, err
}
