/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

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

import (
	"errors"
	"os"
	"reflect"
	"time"
)

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func SliceRemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func SliceSubSlice(src []string, dst []string) []string {
	result := []string{}
	for _, item := range src {
		if !ContainsString(dst, item) {
			result = append(result, item)
		}
	}
	return result
}

func SliceMergeSlice(src []string, dst []string) []string {
	result := []string{}
	tmp := map[string]bool{}
	for _, s := range src {
		tmp[s] = true
	}

	for _, d := range dst {
		tmp[d] = true
	}
	for k, _ := range tmp {
		result = append(result, k)
	}
	return result
}

func SliceEqualSlice(src, dst []string) bool {
	if len(src) != len(dst) {
		return false
	}

	for _, s := range src {
		if !ContainsString(dst, s) {
			return false
		}
	}
	return true
}

func MapEqualMap(src, dst map[string]string) bool {
	if len(src) != len(dst) {
		return false
	}
	for key, value := range src {
		if v, ok := dst[key]; !ok || value != v {
			return false
		}
	}
	return true
}

func FileExists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func UntilMaxRetry(f func() error, maxRetry int, interval time.Duration) error {
	var err error
	for i := 0; i < maxRetry; i++ {

		err = f()

		if err == nil {
			return nil
		}
		time.Sleep(interval)
	}
	return err
}

func Fill(src interface{}, dst interface{}) error {
	srcType := reflect.TypeOf(src)
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst)

	if srcType.Kind() != reflect.Struct {
		return errors.New("src must be  a struct")
	}
	if dstValue.Kind() != reflect.Ptr {
		return errors.New("dst must be a point")
	}

	for i := 0; i < srcType.NumField(); i++ {
		dstField := dstValue.Elem().FieldByName(srcType.Field(i).Name)
		if dstField.CanSet() {
			dstField.Set(srcValue.Field(i))
		}
	}

	return nil
}
