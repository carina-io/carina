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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/carina-io/carina"
	v1 "k8s.io/api/core/v1"
	"os"
	"reflect"
	"strings"
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

func exists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, false
	}
	return info, true
}

// FileExists checks if a file exists and is not a directory
func FileExists(filepath string) bool {
	info, present := exists(filepath)
	return present && info.Mode().IsRegular()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, present := exists(path)
	return present && info.IsDir()
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

	dstValue := reflect.ValueOf(dst)

	if srcType.Kind() != reflect.Struct {
		return errors.New("src must be  a struct")
	}
	if dstValue.Kind() != reflect.Ptr {
		return errors.New("dst must be a point")
	}

	jsonStu, err := json.Marshal(src)
	if err != nil {
		return errors.New("json Marshal fail")
	}
	return json.Unmarshal(jsonStu, &dst)

}

func PartitionName(lv string) string {
	strtemp := strings.Split(lv, "-")
	return fmt.Sprintf("%s/%s", carina.CarinaPrefix, strtemp[len(strtemp)-1])
}

// IsStaticPod returns true if the pod is a static pod.
func IsStaticPod(pod *v1.Pod) bool {
	source, err := GetPodSource(pod)
	return err == nil && source != carina.ApiserverSource
}

// GetPodSource returns the source of the pod based on the annotation.
func GetPodSource(pod *v1.Pod) (string, error) {
	if pod.Annotations != nil {
		if source, ok := pod.Annotations[carina.ConfigSourceAnnotationKey]; ok {
			return source, nil
		}
	}
	return "", fmt.Errorf("cannot get source of pod %q", pod.UID)
}
