package singleflight

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/pkg/errors"
)

type call struct {
	mu *sync.Mutex
	wg *sync.WaitGroup
	w  *httptest.ResponseRecorder
	mw *multiWriter
	n  int64
}

type multiWriter struct {
	writers []http.ResponseWriter
}

func (c *call) addResponseWriter(w http.ResponseWriter) {
	c.mu.Lock()
	c.mw.writers = append(c.mw.writers, w)
	c.n++
	c.mu.Unlock()
}

func (c *call) new() {
	c.reset()
	c.mu = &sync.Mutex{}
	c.wg = &sync.WaitGroup{}
	c.mw = &multiWriter{
		writers: make([]http.ResponseWriter, 0, 10),
	}
	c.w = httptest.NewRecorder()
}

func (c *call) reset() {
	c.mu = nil
	c.wg = nil
	c.w = nil
	c.mw = nil
	c.n = 0
}

func (c *call) flush() {

	resp := c.w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("read response body error:", err)
		return
	}
	header := map[string][]string(resp.Header.Clone())
	statusCode := resp.StatusCode

	if int(c.n) != len(c.mw.writers) {
		log.Printf("length: %d, %d\n", c.n, len(c.mw.writers))
	}

	for _, w := range c.mw.writers {
		w.WriteHeader(statusCode)
		w.Write(body)
		for h, v := range header {
			for _, vv := range v {
				w.Header().Add(h, vv)
			}
		}
	}
}

// Group group
type Group interface {
	Do(next http.Handler, opts ...Option) http.Handler
}

// New new group
func New() Group {
	return &group{
		m: make(map[string]*call),
		callPool: &sync.Pool{
			New: func() interface{} {
				return new(call)
			},
		},
		buffPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 4096))
			},
		},
	}
}

type group struct {
	mu       sync.Mutex
	m        map[string]*call
	callPool *sync.Pool
	buffPool *sync.Pool
}

// Do do
func (g *group) Do(next http.Handler, opts ...Option) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		opt := options{"", ""}

		for _, o := range opts {
			o.apply(&opt)
		}

		if !g.isSingleRequest(opt, r) {
			next.ServeHTTP(w, r)
			return
		}

		err := g.do(w, r, next)
		if err != nil {
			log.Println("group do error:", err)
		}
	})
}

func (g *group) do(w http.ResponseWriter, r *http.Request, next http.Handler) error {

	key, err := g.requestKey(r)
	if err != nil {
		return err
	}

	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		c.addResponseWriter(w)
		g.mu.Unlock()
		c.wg.Wait()
		return nil
	}

	c := g.callPool.Get().(*call)
	c.new()
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()
	c.addResponseWriter(w)

	next.ServeHTTP(c.w, r)

	g.mu.Lock()
	// 保证在同步响应时不会添加新的请求
	c.flush()
	c.wg.Done()
	delete(g.m, key)
	g.callPool.Put(c)
	g.mu.Unlock()

	return nil
}

func (g *group) requestKey(r *http.Request) (string, error) {

	var (
		key    string
		method string
		path   string
		body   string
		header string
		proto  string
	)

	proto = r.Proto
	path = r.RequestURI
	method = r.Method

	headers := map[string][]string(r.Header)
	for k, v := range headers {
		s := ""
		for _, vv := range v {
			s += vv
		}
		header += k + "=" + s + ";"
	}

	if r.Body != nil {
		// buffer := bytes.NewBuffer(make([]byte, 4096))
		buffer := g.buffPool.Get().(*bytes.Buffer)
		// 不知为啥，在 Put 前会造成阻塞
		buffer.Reset()
		_, err := io.Copy(buffer, r.Body)
		if err != nil {
			g.buffPool.Put(buffer)
			return "", errors.Wrap(err, "get request key")
		}
		body = mmd5(buffer.Bytes())
		g.buffPool.Put(buffer)
	}

	key = method + path + proto + header + body

	return key, nil
}

func (g *group) isSingleRequest(opt options, r *http.Request) bool {
	if len(opt.headerName) == 0 ||
		len(opt.headerValue) == 0 {
		return false
	}

	if r.Header.Get(opt.headerName) !=
		opt.headerValue {
		return false
	}
	return true
}

func mmd5(body []byte) string {
	hasher := md5.New()
	hasher.Write(body)
	return hex.EncodeToString(hasher.Sum(nil))
}

// Option option
type Option interface {
	apply(*options)
}

type optionFunc func(*options)

func (f optionFunc) apply(o *options) {
	f(o)
}

// WithHeader 只有 name 和 value 都符合的请求才会被执行
// name 和 value 任意为空都不会执行
func WithHeader(name, value string) Option {
	return optionFunc(func(o *options) {
		o.headerName = name
		o.headerValue = value
	})
}

type options struct {
	headerName  string
	headerValue string
}
