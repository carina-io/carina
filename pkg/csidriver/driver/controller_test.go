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
package driver

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConvertRequestCapacity(t *testing.T) {
	table := []struct {
		requestBytes int64
		limitBytes   int64
		result       int64
		err          error
	}{
		{requestBytes: -1, limitBytes: 0, result: 0, err: errors.New("required")},
		{requestBytes: 41, limitBytes: -1, result: 0, err: errors.New("limit")},
		{requestBytes: 15, limitBytes: 12, result: 0, err: errors.New("exceeds")},
		{requestBytes: 15 << 30, limitBytes: 20 << 30, result: 15, err: nil},
		{requestBytes: 0, limitBytes: 20, result: 1, err: nil},
	}

	a := assert.New(t)

	for _, e := range table {
		v, err := convertRequestCapacity(e.requestBytes, e.limitBytes)
		a.Equal(v, e.result)
		if err != nil {
			a.Contains(err.Error(), e.err.Error())
		}
	}

}
