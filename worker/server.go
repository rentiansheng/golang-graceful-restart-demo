package main

import (
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {

	file := os.NewFile(3, "")
	l, err := net.FileListener(file)
	if err != nil {
		log.Fatal(err)
		return
	}

	done := make(chan bool, 1000)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, path:%s", html.EscapeString(r.URL.Path))
	})
	http.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, pid:%d, time:%s", os.Getpid(), time.Now().String())
		fmt.Println(os.Getpid())
		done <- true

	})

	go func() {
		log.Fatal(http.Serve(l, nil))
		done <- true
		return
	}()
	select {
	case <-done:
	}

}
