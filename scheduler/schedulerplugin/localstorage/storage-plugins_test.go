package localstorage

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMinimumValueMinus(t *testing.T) {
	table := []struct{
		array []int64
		value int64
		result []int64
	}{
		{array:[]int64{3,4,5,2,5,23,1}, value: 3, result: []int64{1, 2, 0, 4, 5, 5, 23}},
		{array:[]int64{3,4,5,2,5,23,1}, value: 33, result: []int64{}},
	}

	a := assert.New(t)
	for _, e := range table {
		a.Equal(minimumValueMinus(e.array, e.value), e.result)
	}
}

func TestReasonableScore(t *testing.T) {
	table := []struct{
		ration int64
		result int64
	} {
		{ration: 100, result:5},{ration: 5, result:2},{ration: 0, result:1},
	}
	a := assert.New(t)
	for _, e := range table {
		a.Equal(reasonableScore(e.ration), e.result)
	}
}