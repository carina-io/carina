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
package localstorage

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMinimumValueMinus(t *testing.T) {
	table := []struct {
		array  []int64
		value  int64
		result []int64
	}{
		{array: []int64{3, 4, 5, 2, 5, 23, 1}, value: 3, result: []int64{1, 2, 0, 4, 5, 5, 23}},
		{array: []int64{3, 4, 5, 2, 5, 23, 1}, value: 33, result: []int64{}},
	}

	a := assert.New(t)
	for _, e := range table {
		minimumValueMinus(e.array, &pvcRequest{exclusive: false, request: e.value})
		a.Equal(e.array, e.result)
	}
}
