// Copyright 2024 Linka Cloud  All rights reserved.
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

package pm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := New(ctx, "")
	do := func() {
		require.NoError(t, m.AddFunc("whatever", func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}))
		errs := m.RunBackground(ctx)
		select {
		case <-errs:
			require.Fail(t, "should not return")
		default:
		}
	}
	do()
	time.Sleep(time.Millisecond)
	require.NoError(t, m.Close())
	do()
}
