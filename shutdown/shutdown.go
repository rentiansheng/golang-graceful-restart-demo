package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	svrAddr := ":8080"

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)

	//创建 server：
	server := newWebServer(logger, svrAddr)

	done := make(chan struct{}, 1)

	//启动另一个 goroutine ，监听将要关闭信号：
	go shutdown(server, logger, done)

	go func() {
		//启动 server：
		logger.Println("Server address", svrAddr)
		err := server.ListenAndServe()
		// http.ErrServerClosed 是有http shutdown 引起
		if err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen %s error. err:%s\n", svrAddr, err)
		}

	}()

	//等待已经关闭的信号：
	<-done
	logger.Println("Server stopped")
}

//初始化 server
func newWebServer(logger *log.Logger, svrAddr string) *http.Server {
	// http server路由
	router := http.NewServeMux()
	router.HandleFunc("/", sleepRequestHandle)

	//http 服务配置:
	server := &http.Server{
		Addr:         svrAddr,
		Handler:      router,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 20 * time.Minute,
		IdleTimeout:  10 * time.Minute,
	}

	return server
}

// shutdown 关闭 server
// parameter done: 发出已经关闭信号
func shutdown(server *http.Server, logger *log.Logger, done chan<- struct{}) {
	// 等待Ctrl+c 的新号
	quit := make(chan os.Signal, 1)

	signal.Notify(quit)
	//等待接收到退出信号：
	<-quit
	logger.Println("Server is shutting down...")

	// WithTimeout.WithTimeOut  shutdown 等待时间
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	err := server.Shutdown(ctx)
	if err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v \n", err)
	}

	close(done)
}

// sleepRequestHandle 等两分钟后退出。 看下http.server 的showdown 是否有效
func sleepRequestHandle(w http.ResponseWriter, r *http.Request) {
	log.Println("waiting ")
	time.Sleep(time.Minute * 1)
	strBody := fmt.Sprintf(" current:%v", time.Now().Format("2006-01-02 15:04:05"))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strBody))
}
