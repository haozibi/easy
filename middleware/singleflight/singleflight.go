package singleflight

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/pkg/errors"
)

// 多个相同请求，其中一个访问真正的后端，其他请求等待其完成
// 反向代理

type call struct {
	mu sync.Mutex
	wg sync.WaitGroup
	w  *httptest.ResponseRecorder
	mw *multiWriter
}

type multiWriter struct {
	writers []http.ResponseWriter
}

func (c *call) addResponseWriter(w http.ResponseWriter) {
	c.mu.Lock()
	if c.mw.writers == nil {
		c.mw.writers = make([]http.ResponseWriter, 0)
	}
	c.mw.writers = append(c.mw.writers, w)
	c.mu.Unlock()
}

func (c *call) reset() {
	c.w = nil
	c.mw = nil
}

func (c *call) flush() {

	resp := c.w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	header := map[string][]string(resp.Header.Clone())
	statusCode := resp.StatusCode

	for _, w := range c.mw.writers {
		w.Write(body)
		w.WriteHeader(statusCode)
		for k, v := range header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
	}
}

// Group group
type Group struct {
	mu       sync.Mutex
	m        map[string]*call
	callPool *sync.Pool
	buffPool *sync.Pool
}

// Do do
func (g *Group) Do(next http.Handler, opts ...Option) http.Handler {
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
			panic(err)
		}
	})
}

func (g *Group) do(w http.ResponseWriter, r *http.Request, next http.Handler) error {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if g.callPool == nil {
		g.callPool = &sync.Pool{
			New: func() interface{} {
				return new(call)
			},
		}
	}
	if g.buffPool == nil {
		g.buffPool = &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 4096))
			},
		}
	}
	key, err := g.requestKey(r)
	if err != nil {
		return err
	}

	// 只保证在首个请求进行请求时进入的相同请求会被阻塞
	// 返回相同响应
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		// 是否并发安全，遗漏响应
		c.addResponseWriter(w)
		c.wg.Wait()
		return nil
	}

	c := g.callPool.Get().(*call)
	c.wg.Add(1)
	g.m[key] = c
	c.mw = new(multiWriter)
	c.w = httptest.NewRecorder()
	c.addResponseWriter(w)
	g.mu.Unlock()

	// 唯一请求转发
	// todo: 利用 channel 把这段分离，避免耦合
	next.ServeHTTP(c.w, r)

	// 把请求响应统一复制到所有等待的响应
	c.flush()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	// c.reset()
	g.callPool.Put(c)
	g.mu.Unlock()

	return nil
}

func (g *Group) requestKey(r *http.Request) (string, error) {

	var (
		key    string
		method string
		path   string
		body   string
		header string
		proto  string
	)

	proto = r.Proto
	path = r.URL.RawPath
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

func (g *Group) isSingleRequest(opt options, r *http.Request) bool {
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
