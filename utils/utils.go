package utils

import (
	"os"
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
