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
	"sync"
)

type notifierKey struct{}

func notifyCtx(ctx context.Context, fn func(s Status)) (context.Context, func()) {
	n := &notifier{c: make(chan Status)}
	ctx = context.WithValue(ctx, notifierKey{}, n)
	n.run(fn)
	return ctx, n.close
}

func Notify(ctx context.Context, s Status) {
	n, ok := ctx.Value(notifierKey{}).(*notifier)
	if !ok {
		return
	}
	select {
	case <-n.c:
	case n.c <- s:
	default:
	}
}

type notifier struct {
	o sync.Once
	c chan Status
}

func (n *notifier) run(fn func(s Status)) {
	go func() {
		for s := range n.c {
			fn(s)
		}
	}()
}

func (n *notifier) close() {
	n.o.Do(func() {
		close(n.c)
	})
}
