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

package getter

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RetryGetter struct {
	cache client.Reader
	api   client.Reader
}

func NewRetryGetter(mgr manager.Manager) *RetryGetter {
	return &RetryGetter{
		cache: mgr.GetClient(),
		api:   mgr.GetAPIReader(),
	}
}

// Get try to get from cache at first, then api
func (r *RetryGetter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	err := r.cache.Get(ctx, key, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return r.api.Get(ctx, key, obj)
}
