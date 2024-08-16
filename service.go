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
	"fmt"

	"github.com/thejerf/suture/v4"
)

type ServiceFunc func(ctx context.Context) error

func NewServiceFunc(name string, f ServiceFunc) Service {
	return &serviceFunc{name: name, f: f}
}

type serviceFunc struct {
	name string
	f    ServiceFunc
}

func (s *serviceFunc) Serve(ctx context.Context) error {
	return s.f(ctx)
}

func (s *serviceFunc) String() string {
	return s.name
}

type Service interface {
	suture.Service
	fmt.Stringer
}
