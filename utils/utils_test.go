package utils

import (
	"github.com/carina-io/carina"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestContainsString(t *testing.T) {
	table := []struct {
		slice  []string
		s      string
		result bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
	}

	for _, e := range table {
		if ContainsString(e.slice, e.s) != e.result {
			t.Errorf("ContainsString(%v, %s)", e.slice, e.s)
		}
	}
}

func TestSliceRemoveString(t *testing.T) {
	table := []struct {
		slice  []string
		s      string
		result []string
	}{
		{slice: []string{"a", "b", "c"}, s: "a", result: []string{"b", "c"}},
		{slice: []string{"a", "b", "c"}, s: "d", result: []string{"a", "b", "c"}},
	}

	a := assert.New(t)

	for _, e := range table {
		a.Equal(SliceRemoveString(e.slice, e.s), e.result)
	}
}

func TestSliceSubSlice(t *testing.T) {
	table := []struct {
		src    []string
		dst    []string
		result []string
	}{
		{src: []string{"a", "b", "c"}, dst: []string{"a", "b"}, result: []string{"c"}},
		{src: []string{"a", "b", "c"}, dst: []string{"a", "d"}, result: []string{"b", "c"}},
	}
	a := assert.New(t)
	for _, e := range table {
		a.Equal(SliceSubSlice(e.src, e.dst), e.result)
	}
}

func TestSliceMergeSlice(t *testing.T) {
	table := []struct {
		src    []string
		dst    []string
		result []string
	}{
		{src: []string{"a", "b", "c"}, dst: []string{"a", "b"}, result: []string{"a", "b", "c"}},
		{src: []string{"a", "b", "c"}, dst: []string{"a", "d"}, result: []string{"a", "b", "c", "d"}},
	}
	a := assert.New(t)
	for _, e := range table {
		a.Equal(SliceEqualSlice(SliceMergeSlice(e.src, e.dst), e.result), true)
	}
}

func TestSliceEqualSlice(t *testing.T) {
	table := []struct {
		src    []string
		dst    []string
		result bool
	}{
		{src: []string{"a", "b", "c"}, dst: []string{"a", "b"}, result: false},
		{src: []string{"a", "b", "c"}, dst: []string{"a", "b", "c"}, result: true},
	}
	a := assert.New(t)
	for _, e := range table {
		a.Equal(SliceEqualSlice(e.src, e.dst), e.result)
	}
}

func TestMapEqualMap(t *testing.T) {
	table := []struct {
		src    map[string]string
		dst    map[string]string
		result bool
	}{
		{src: map[string]string{"a": "b"}, dst: map[string]string{"a": "b"}, result: true},
		{src: map[string]string{}, dst: map[string]string{"c": "d", "a": "1"}, result: false},
	}
	a := assert.New(t)
	for _, e := range table {
		a.Equal(MapEqualMap(e.src, e.dst), e.result)
	}
}

func TestIsStaticPod(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Annotations: map[string]string{
				carina.ConfigSourceAnnotationKey: "file",
			},
		},
	}

	assert.New(t).True(IsStaticPod(pod))
}
