package main

import (
	"log"
	"time"

	"github.com/haozibi/easy"
)

func main() {

	e := easy.New(
		easy.WithReadTimeout(5*time.Second),
		easy.WithWriteTimeout(5*time.Second),
	)
	if err := e.ListenAndServe(":9191"); err != nil {
		log.Fatalln(err)
	}
}
