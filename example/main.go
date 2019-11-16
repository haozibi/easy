package main

import (
	"github.com/haozibi/easy"
	"log"
)

func main() {

	e := easy.New()
	if err := e.ListenAndServe(":9191"); err != nil {
		log.Fatalln(err)
	}
}
