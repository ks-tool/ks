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
	"time"

	resourcemanagerv1 "github.com/yandex-cloud/go-genproto/yandex/cloud/resourcemanager/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
	"github.com/yandex-cloud/go-sdk/sdkresolvers"
)

type ClientConfig func(*Client)

type Client struct {
	sdk      *ycsdk.SDK
	folderId string
	cloudId  string
}

// NewFromToken creates an SDK instance with credentials for user Yandex Passport OAuth token.
// See https://cloud.yandex.ru/docs/iam/concepts/authorization/oauth-token for details.
func NewFromToken(token string, opts ...ClientConfig) (c *Client, err error) {
	if len(token) == 0 {
		return nil, errors.New("token required")
	}

	sdk, _ := ycsdk.Build(nil, ycsdk.Config{
		Credentials: ycsdk.OAuthToken(token),
	})
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()

	c = &Client{sdk: sdk}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// NewFromIAMKey creates an SDK instance with credentials for the given IAM Key
// See https://yandex.cloud/ru/docs/iam/concepts/authorization/iam-token for details.
func NewFromIAMKey(token []byte, opts ...ClientConfig) (c *Client, err error) {
	key, err := iamkey.ReadFromJSONBytes(token)
	if err != nil {
		return nil, err
	}

	cred, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		return nil, err
	}

	sdk, _ := ycsdk.Build(nil, ycsdk.Config{
		Credentials: cred,
	})
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()

	c = &Client{sdk: sdk}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func NewClient(token string, opts ...ClientConfig) (*Client, error) {
	if len(opts) == 0 {
		opts = append(opts, FolderName("default"))
	}
	if token[:2] == "t1." {
		return NewFromIAMKey([]byte(token), opts...)
	}

	return NewFromToken(token, opts...)
}

func (c *Client) CloudId() string  { return c.cloudId }
func (c *Client) FolderId() string { return c.folderId }
func (c *Client) SDK() *ycsdk.SDK  { return c.sdk }

func CloudId(id string) ClientConfig {
	return func(c *Client) {
		c.cloudId = id
	}
}

func CloudName(name string) ClientConfig {
	return func(c *Client) {
		res := sdkresolvers.CloudResolver(name)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := res.Run(ctx, c.sdk); err != nil {
			panic(err)
		}

		if res.Err() != nil {
			panic(res.Err())
		}

		c.cloudId = res.ID()
	}
}

func FolderId(id string) ClientConfig {
	return func(c *Client) {
		c.folderId = id
	}
}

func FolderName(name string) ClientConfig {
	return func(c *Client) {
		if len(c.cloudId) == 0 {
			func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				lst, err := c.sdk.ResourceManager().Cloud().List(ctx, &resourcemanagerv1.ListCloudsRequest{})
				if err != nil {
					panic(err)
				}
				for _, cloud := range lst.Clouds {
					c.cloudId = cloud.Id
					break
				}
			}()
		}
		res := sdkresolvers.FolderResolver(name, sdkresolvers.CloudID(c.cloudId))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := res.Run(ctx, c.sdk); err != nil {
			panic(err)
		}

		if res.Err() != nil {
			panic(res.Err())
		}

		c.folderId = res.ID()
	}
}
