package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/haozibi/easy/middleware/singleflight"
)

var count int64
var g singleflight.Group = singleflight.New()

func greet(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&count, 1)
	time.Sleep(20 * time.Millisecond)
	fmt.Fprintf(w, "Hello World! %s", time.Now())
}

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, count)
}

func main() {
	http.Handle("/single", g.Do(http.HandlerFunc(greet), singleflight.WithHeader("x-fly", "abc")))
	http.HandleFunc("/original", greet)
	http.HandleFunc("/count", index)

	addr := ":9091"
	fmt.Println("Listen:", addr)
	http.ListenAndServe(addr, nil)
}
