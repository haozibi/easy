package easy

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// todo
// 1. 路由的实现
// 2. 希望可以直接从解析出数据
// 3. 支持 fasthttp 后端

// Easy easy
type Easy struct {
	*mux.Router
	srv *http.Server

	shutdown         chan struct{}
	shutdownComplete chan struct{}
}

// New new easy
func New(opts ...Option) *Easy {
	e := &Easy{
		Router:           mux.NewRouter(),
		shutdown:         make(chan struct{}),
		shutdownComplete: make(chan struct{}),
	}

	e.new(opts...)
	return e
}

func (e *Easy) new(opts ...Option) {
	opt := e.defaultOptions()

	for _, o := range opts {
		o.apply(&opt)
	}

	e.printOption(opt)
	e.srv = &http.Server{
		ReadTimeout:    opt.readTimeout,
		WriteTimeout:   opt.writeTimeout,
		MaxHeaderBytes: opt.maxHeaderBytes,
		Handler:        handlers.RecoveryHandler()(e),
	}
	e.Router.MethodNotAllowedHandler = opt.methodNotAllowedHandler
	e.Router.NotFoundHandler = opt.notFoundHandler
}

func (e *Easy) defaultOptions() options {
	return options{
		readTimeout:    3 * time.Second,
		writeTimeout:   3 * time.Second,
		maxHeaderBytes: http.DefaultMaxHeaderBytes,

		// NotFoundHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// }),
		// MethodNotAllowedHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// }),
	}
}

func (e *Easy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.Router.ServeHTTP(w, r)
}

// ListenAndServe run
func (e *Easy) ListenAndServe(address string) (err error) {
	go e.quit()

	e.srv.Addr = address
	log.Printf("Listen Address: %s\n", address)
	err = e.srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return
	}
	log.Println("Server exit success")
	return nil
}

// Shutdown Shutdown
func (e *Easy) quit() {

	quit := make(chan os.Signal)
	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	<-quit
	log.Println("Shutdown server")
	close(e.shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.srv.Shutdown(ctx); err != nil {
		log.Println("Server Shutdown: ", err)
	}

	log.Println("Server exiting")
}

func (e *Easy) printOption(opt options) {

	log.Printf("Read Timeout: %s\n", opt.readTimeout)
	log.Printf("Write Timeout: %s\n", opt.writeTimeout)
}

type options struct {
	readTimeout    time.Duration
	writeTimeout   time.Duration
	maxHeaderBytes int

	notFoundHandler         http.Handler
	methodNotAllowedHandler http.Handler
}

// Option option
type Option interface {
	apply(*options)
}

type optionFunc func(*options)

func (f optionFunc) apply(o *options) {
	f(o)
}

// WithReadTimeout set server read time out
func WithReadTimeout(t time.Duration) Option {
	return optionFunc(func(o *options) {
		o.readTimeout = t
	})
}

// WithWriteTimeout set server write time out
func WithWriteTimeout(t time.Duration) Option {
	return optionFunc(func(o *options) {
		o.writeTimeout = t
	})
}

// WithMaxHeaderBytes set server max header size
func WithMaxHeaderBytes(n int) Option {
	return optionFunc(func(o *options) {
		o.maxHeaderBytes = n
	})
}

// WithNotFoundHandler set server 404 Handler
func WithNotFoundHandler(h http.Handler) Option {
	return optionFunc(func(o *options) {
		o.notFoundHandler = h
	})
}

// WithMethodNotAllowedHandler set server 405 Handler
func WithMethodNotAllowedHandler(h http.Handler) Option {
	return optionFunc(func(o *options) {
		o.methodNotAllowedHandler = h
	})
}
