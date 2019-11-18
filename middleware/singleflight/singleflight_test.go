package singleflight

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleGroup(t *testing.T) {

	var (
		g           Group
		calls       int32
		wg          sync.WaitGroup
		n           = 10
		c           = make(chan string)
		t1          = time.Now()
		headerName  = "abc"
		headerValue = "abcabc"
	)

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	req.Header.Set(headerName, headerValue)

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		io.WriteString(w, <-c)
	})
	handler = g.Do(handler, WithHeader(headerName, headerValue))

	for i := 0; i < n; i++ {
		wg.Add(1)
		i := i
		go func() {
			fmt.Println("req num:", i)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			respBody, _ := ioutil.ReadAll(resp.Body)

			fmt.Println(i, resp.StatusCode, string(respBody))
			wg.Done()
		}()

	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("===")
	c <- "bar"
	// close(c)
	wg.Wait()

	got := atomic.LoadInt32(&calls)

	fmt.Printf("calls: %d,time: %v\n", got, time.Since(t1))
}
