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

func (c *Client) FirstSubnetInZone(ctx context.Context, folder string, zone string) (string, error) {
	var folderId string
	if len(folder) == 0 || folder == c.folder {
		folderId = c.folderId
	} else {
		cctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		fld := sdkresolvers.FolderResolver(folder, sdkresolvers.CloudID(c.cloudId))
		if err := fld.Run(cctx, c.sdk); err != nil {
			return "", err
		}
		if fld.Err() != nil {
			return "", fld.Err()
		}
		folderId = fld.ID()
	}

	cctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	iter := c.sdk.VPC().Subnet().SubnetIterator(cctx, &vpc.ListSubnetsRequest{
		FolderId: folderId,
	})

	for iter.Next() {
		subnet := iter.Value()
		if subnet.ZoneId == zone {
			return subnet.Id, nil
		}
	}

	if err := iter.Error(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("no subnet in zone %q", zone)
}
