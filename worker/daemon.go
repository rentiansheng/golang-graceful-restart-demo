package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
)

func main() {
	fmt.Println("start")

	listen, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Fatal(err)
	}
	l := listen.(*net.TCPListener)
	listenFile, err := l.File()
	if err != nil {
		log.Fatal(err)
	}

	for {
		fmt.Println("execute start")

		execFile := "./server"
		execCmd := exec.Command(execFile)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.ExtraFiles = []*os.File{listenFile}
		err := execCmd.Run()
		if err != nil {
			log.Fatalf(err.Error())
		}

		fmt.Println("execute end")
	}

}
