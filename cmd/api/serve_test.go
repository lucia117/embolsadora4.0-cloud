package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func startServe(t *testing.T, handler http.Handler, timeout time.Duration) (addr string, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan error, 1)
	srv := &http.Server{Handler: handler}
	go func() { ch <- serve(ctx, srv, ln, timeout) }()
	return "http://" + ln.Addr().String(), cancel, ch
}

// Un request en curso cuando llega la señal termina bien: es el caso de un
// batch de ingesta a medio escribir cuando Cloud Run manda SIGTERM.
func TestServe_InFlightRequestCompletesOnShutdown(t *testing.T) {
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	})
	addr, cancel, done := startServe(t, handler, 5*time.Second)

	type result struct {
		body string
		err  error
	}
	res := make(chan result, 1)
	go func() {
		resp, err := http.Get(addr)
		if err != nil {
			res <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		res <- result{body: string(b), err: err}
	}()

	<-started
	cancel() // llega la señal con el request todavía en curso

	r := <-res
	if r.err != nil || r.body != "ok" {
		t.Fatalf("el request en curso debía completarse: body=%q err=%v", r.body, r.err)
	}
	if err := <-done; err != nil {
		t.Fatalf("serve debía terminar sin error, devolvió: %v", err)
	}
}

// Una conexión que nunca queda inactiva (SSE) no puede bloquear el apagado
// más allá del timeout: se corta con Close y serve termina igual.
func TestServe_ForcesCloseAfterTimeout(t *testing.T) {
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done() // como el stream SSE: solo termina si cortan la conexión
	})
	addr, cancel, done := startServe(t, handler, 200*time.Millisecond)

	go func() {
		resp, err := http.Get(addr)
		if err == nil {
			resp.Body.Close()
		}
	}()

	<-started
	begin := time.Now()
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("serve debía informar el timeout de Shutdown, devolvió: %v", err)
		}
		if elapsed := time.Since(begin); elapsed > 2*time.Second {
			t.Fatalf("el apagado tardó %v; debía cortar al cumplirse el timeout", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve no terminó: una conexión larga bloqueó el apagado")
	}
}

// Si el listener falla (por ejemplo, el puerto ya está en uso), serve devuelve
// ese error sin esperar ninguna señal.
func TestServe_ReturnsListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ln.Close() // Serve sobre un listener cerrado falla de inmediato

	err = serve(context.Background(), &http.Server{}, ln, time.Second)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("serve debía devolver el error del listener, devolvió: %v", err)
	}
}
