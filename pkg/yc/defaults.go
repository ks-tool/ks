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
	"time"
)

const (
	KsToolKey = "yc.ks-tool.dev"

	DefaultDiskType          = "network-ssd"
	DefaultDiskID            = "fd83j4siasgfq4pi1qif" //debian-12-v20240920
	DefaultDiskSizeGib int64 = 10

	DefaultPlatformID = "standard-v3"
	DefaultZone       = "ru-central1-d"

	DefaultCores        int64 = 2
	DefaultCoreFraction int64 = 100
	DefaultMemoryGib    int64 = 2

	requestTimeout = 15 * time.Second
)
