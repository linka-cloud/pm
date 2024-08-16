package pm

import (
	"context"
	"errors"

	"go.linka.cloud/grpc-toolkit/logger"
)

type Wrapper interface {
	Unwrap() Service
}

type wrapper struct {
	s Service
	m *manager
}

func (w *wrapper) Serve(ctx context.Context) error {
	w.m.setStatus(w.s.String(), StatusStarting)
	ctx, clean := notifyCtx(ctx, func(s Status) {
		w.m.setStatus(w.s.String(), s)
	})
	defer clean()
	if err := w.s.Serve(logger.Set(ctx, logger.C(ctx).WithFields("service", w.s.String()))); err != nil && !errors.Is(err, context.Canceled) {
		w.m.setStatus(w.s.String(), StatusError)
		return err
	}
	w.m.setStatus(w.s.String(), StatusStopped)
	return nil
}

func (w *wrapper) String() string {
	return w.s.String()
}

func (w *wrapper) Unwrap() Service {
	return w.s
}
