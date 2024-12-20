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

package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.linka.cloud/grpc-toolkit/logger"
	"go.linka.cloud/grpc-toolkit/signals"

	"go.linka.cloud/pm"
	"go.linka.cloud/pm/reexec"
)

func init() {
	if reexec.Init() {
		return
	}
}

func main() {
	ctx, cancel := context.WithCancel(signals.SetupSignalHandler())
	defer cancel()
	if err := run(ctx); err != nil {
		logger.C(ctx).Fatal(err)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m := pm.New(ctx, "main")

	m.WatchChanges(context.Background(), func(n string, s pm.Status) {
		logger.C(ctx).WithFields("service", n, "status", s).Info("status changed")
	})

	stopping := pm.NewServiceFunc("stopping", func(ctx context.Context) error {
		pm.Notify(ctx, pm.StatusStarting)
		time.Sleep(time.Second)
		var o sync.Once
		tk := time.NewTicker(1 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				o.Do(func() {
					pm.Notify(ctx, pm.StatusRunning)
				})
			case <-ctx.Done():
				pm.Notify(ctx, pm.StatusStopping)
				time.Sleep(time.Second)
				pm.Notify(ctx, pm.StatusStopped)
				logger.C(ctx).Warnf("terminating service")
				return nil
			}
		}
	})
	if err := m.Add(stopping); err != nil {
		return err
	}
	echoDate := pm.Command("echo date", "sh", "-c", `while true; do echo "date: $(date)"; sleep 1; done`)
	if err := m.Add(echoDate); err != nil {
		return err
	}
	if err := m.AddFunc("failing", func(ctx context.Context) error {
		pm.Notify(ctx, pm.StatusStarting)
		time.Sleep(time.Second)
		logger.C(ctx).Infof("failing in 1 second")
		pm.Notify(ctx, pm.StatusRunning)
		time.Sleep(time.Second)
		return fmt.Errorf("failed")
	}); err != nil {
		return err
	}
	p := pm.NewServiceFunc("panicking", func(ctx context.Context) error {
		pm.Notify(ctx, pm.StatusStarting)
		time.Sleep(time.Second)
		pm.Notify(ctx, pm.StatusRunning)
		time.Sleep(2 * time.Second)
		panic("test panic")
	})
	if err := m.Add(p); err != nil {
		return err
	}

	if err := m.AddFunc("monitor", func(ctx context.Context) error {
		tk := time.NewTicker(10 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				logger.C(ctx).Infof("stopping service")
				if err := m.Stop(stopping); err != nil {
					logger.C(ctx).WithError(err).Errorf("failed to stop service")
				}
				logger.C(ctx).Infof("service stopped")
				time.Sleep(time.Second)
				logger.C(ctx).Infof("starting service")
				if err := m.Add(stopping); err != nil {
					logger.C(ctx).WithError(err).Errorf("failed to start service")
				}
				logger.C(ctx).Info("service started")
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}); err != nil {
		return err
	}

	return m.Run(ctx)
}
