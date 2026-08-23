package app

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func (a *App) Serve(ctx context.Context) error {
	a.StartRecovery(ctx)
	server := &http.Server{Addr: a.Config.HTTPAddr, Handler: a.Handler, ReadHeaderTimeout: a.Config.RequestTimeout}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
