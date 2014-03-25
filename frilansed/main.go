package main

import (
	"os"
	"os/signal"
	"flag"

	"github.com/martinolsen/frilanse"
)

func main() {
	var addr = flag.String("http", ":80", "HTTP listen adddress")

	flag.Parse()

	go frilanse.Start(*addr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	<-sig
	println("Closing...")
}
