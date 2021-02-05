package utils

func IsContainsString(slice []string, s string) bool {
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
		if !IsContainsString(dst, item) {
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
		if !IsContainsString(dst, s) {
			return false
		}
	}
	return true
}
