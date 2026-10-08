package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
)

// shutdownTimeout acota la espera de las requests en curso al apagar. Cloud
// Run da 10 s entre el SIGTERM y el SIGKILL; 8 s deja margen para cerrar Redis
// y el pool de Postgres después.
const shutdownTimeout = 8 * time.Second

// serve atiende srv sobre ln hasta que ctx se cancela (SIGTERM/SIGINT en main)
// y entonces apaga en orden: deja de aceptar conexiones y espera a que
// terminen las requests en curso —un batch de ingesta a medio escribir
// termina en vez de cortarse— durante hasta timeout.
//
// Las conexiones que nunca quedan inactivas (el stream SSE de /logs/stream)
// no liberan a Shutdown por sí solas: al vencer el timeout se cortan con
// Close y serve devuelve context.DeadlineExceeded. Un error del listener se
// devuelve de inmediato, sin esperar señal.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Printf("señal de apagado recibida; esperando requests en curso (hasta %s)", timeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("apagado incompleto (%v); cerrando conexiones restantes", err)
		_ = srv.Close()
		return err
	}
	log.Printf("servidor apagado en orden")
	return nil
}
