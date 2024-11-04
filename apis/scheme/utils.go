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

package scheme

import (
	"os"

	"github.com/mitchellh/go-homedir"
	"k8s.io/apimachinery/pkg/runtime"
)

func Decode(b []byte) (runtime.Object, error) {
	return runtime.Decode(Codecs.UniversalDeserializer(), b)
}

func FromFileWithDefaults(path string) (runtime.Object, error) {
	manifestFilePath, err := homedir.Expand(path)
	if err != nil {
		return nil, err
	}

	manifestBytes, err := os.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	obj, err := Decode(manifestBytes)
	if err != nil {
		return nil, err
	}

	Defaults(obj)

	return obj, nil
}
