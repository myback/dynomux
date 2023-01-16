package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/myback/dynomux"
	"github.com/sirupsen/logrus"
)

type handler func(http.ResponseWriter, *http.Request)

var handleMap = map[string]handler{}

const reservedHostname = "localho.st"

func main() {
	handlers := dynomux.NewServeMux()
	handleMap["new"] = newHandler("new", http.StatusOK)
	handleMap["main"] = newHandler("main", http.StatusCreated)

	if err := handlers.HandleFunc(reservedHostname+"/api/v1/handlers", handleWrapper(handlers)); err != nil {
		logrus.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	srv := &http.Server{Addr: ":8080", Handler: handlers}
	go func() {
		logrus.Info("listen on 8080 port")
		if err := srv.ListenAndServe(); err != nil {
			logrus.Fatal(err)
		}
	}()

	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv.SetKeepAlivesEnabled(false)

	if err := srv.Shutdown(ctx); err != nil {
		logrus.Fatalf("shutdown server error: %s", err)
	}
}

type Request struct {
	Pattern string `json:"p"`
	Handler string `json:"h"`
}

func newHandler(name string, status int) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		host := dynomux.StripHostPort(r.Host)
		echo := fmt.Sprintf("handler name: %s; hostname: %s; method: %s; uri: %s",
			name, host, r.Method, r.RequestURI)

		logrus.Info(echo)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(echo))
	}
}

func handleWrapper(handlers *dynomux.ServeMux) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("in: %s: %s %s", r.Host, r.Method, r.RequestURI)

		switch r.Method {
		case http.MethodPut:
			add(handlers, w, r)
		case http.MethodDelete:
			del(handlers, w, r)
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
	}
}

func add(handlers *dynomux.ServeMux, w http.ResponseWriter, r *http.Request) {
	req, err := parseBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Handler) == 0 {
		http.Error(w, "handler is empty", http.StatusBadRequest)
		return
	}

	h, ok := handleMap[req.Handler]
	if !ok {
		http.Error(w, "unknown handler "+req.Handler, http.StatusBadRequest)
		return
	}

	if err = handlers.HandleFunc(req.Pattern, h); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func del(handlers *dynomux.ServeMux, w http.ResponseWriter, r *http.Request) {
	req, err := parseBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = handlers.RemoveHandler(req.Pattern); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func parseBody(r *http.Request) (Request, error) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}

	if len(req.Pattern) == 0 {
		return req, errors.New("pattern is empty")
	}

	if strings.HasPrefix(req.Pattern, reservedHostname) {
		return req, errors.New(reservedHostname + " - reserved hostname")
	}

	return req, nil
}
