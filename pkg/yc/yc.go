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
	"errors"

	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
)

type Client struct {
	sdk *ycsdk.SDK
}

// NewFromToken creates an SDK instance with credentials for user Yandex Passport OAuth token.
// See https://cloud.yandex.ru/docs/iam/concepts/authorization/oauth-token for details.
func NewFromToken(token string) (*Client, error) {
	if len(token) == 0 {
		return nil, errors.New("token required")
	}

	sdk, _ := ycsdk.Build(nil, ycsdk.Config{
		Credentials: ycsdk.OAuthToken(token),
	})

	return &Client{sdk: sdk}, nil
}

// NewFromIAMKey creates an SDK instance with credentials for the given IAM Key
// See https://yandex.cloud/ru/docs/iam/concepts/authorization/iam-token for details.
func NewFromIAMKey(token []byte) (*Client, error) {
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

	return &Client{sdk: sdk}, nil
}

func NewClient(token string) (*Client, error) {
	if token[:2] == "t1." {
		return NewFromIAMKey([]byte(token))
	}

	return NewFromToken(token)
}
