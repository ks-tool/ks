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

package utils

const Gib int64 = 1024 * 1024 * 1024

func ToGib(v uint) int64 { return int64(v) * Gib }

func AllInMap[T comparable](m1, m2 map[string]T) bool {
	if len(m2) == 0 {
		return true
	}

	for k, v := range m2 {
		if m1[k] != v {
			return false
		}
	}

	return true
}
