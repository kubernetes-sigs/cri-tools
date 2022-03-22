// Copyright 2019 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schedule

import (
	"context"
	"time"
)

// Every will call f periodically.
// The first call will be made immediately.
// Calls are made synchronously, so f will not be executed concurrently.
func Every(ctx context.Context, period time.Duration, f func(context.Context)) {
	if ctx.Err() != nil {
		return
	}
	// Run f immediately, then periodically call it again.
	t := time.NewTicker(period)
	defer t.Stop()
	f(ctx)
	for {
		select {
		case <-t.C:
			f(ctx)
		case <-ctx.Done():
			return
		}
	}
}
