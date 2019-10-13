package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var (
	// 等待更新命令
	graceful = make(chan bool, 1)
	// http server 退出通知,后续可以使用http shutdown
	done = make(chan bool, 1000)
)

func main() {
	fmt.Println("start")

	var graceful bool
	var startTime string
	flag.BoolVar(&graceful, "graceful", false, "graceful start")
	flag.StringVar(&startTime, "start", "", "graceful start")
	flag.Parse()

	server := &http.Server{Addr: ":9999"}

	var listenFile *os.File
	if graceful {
		// why fd = 3. 
		// ExtraFiles specifies additional open files to be inherited by the
        // new process. It does not include standard input, standard output, or
        // standard error. If non-nil, entry i becomes file descriptor 3+i.
		listenFile = os.NewFile(3, "")
	} else {

		listen, err := net.Listen("tcp", ":9999")
		if err != nil {
			log.Fatal(err)
		}
		l := listen.(*net.TCPListener)
		listenFile, err = l.File()
		if err != nil {
			log.Fatal(err)
		}
	}
	go execWorker(listenFile)
	httServer(listenFile, server)

}

func execWorker(listenFile *os.File) {
	<-graceful
	fmt.Println("execute start")

	execFile := "./graceful"
	execCmd := exec.Command(execFile, "-graceful=true", "-start="+time.Now().Format(time.RFC3339))
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.ExtraFiles = []*os.File{listenFile}
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	err := execCmd.Run()
	done <- true
	fmt.Println("execute end", err)
}

func httServer(file *os.File, server *http.Server) {

	l, err := net.FileListener(file)
	if err != nil {
		log.Fatal(err)
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, pid:%d, path:%s, time:%s",os.Getpid(), html.EscapeString(r.URL.Path), time.Now().String())
	})
	http.HandleFunc("/graceful", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, graceful  pid:%s, time:%s", os.Getpid(),time.Now().String())
		close(graceful)
	})
	http.HandleFunc("/sleep", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Second)
		fmt.Fprintf(w, "Hello, graceful pid:%d,  time:%s", os.Getpid(),time.Now().String())
	})

	go func() {
		err := server.Serve(l)
		// http.ErrServerClosed 是有http shutdown 引起
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen %s error. err:%s\n", server.Addr, err)
		}
		return
	}()
	select {
	case <-done:
	case <-shutdown(server):
	}

}

func shutdown(server *http.Server) chan bool {

	<-graceful

	log.Println("Server is shutting down...")

	// WithTimeout.WithTimeOut  shutdown 等待时间
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	//defer cancel()

	server.SetKeepAlivesEnabled(false)
	err := server.Shutdown(context.Background())
	if err != nil {
		log.Fatalf("Could not gracefully shutdown the server: %v \n", err)
	}
	quitChan := make(chan bool, 1)
	quitChan <- true
	return quitChan

}
