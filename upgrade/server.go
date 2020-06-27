package upgrade

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// StartServers Serve announces on the local network address.
//
// The network must be "tcp", "tcp4", "tcp6".
//  "unix", "unixpacket", "udp", "udp4", "udp6", "unixgram" unimplement
//
// For TCP networks, if the host in the address parameter is empty or
// a literal unspecified IP address, Listen listens on all available
// unicast and anycast IP addresses of the local system.
// To only use IPv4, use network "tcp4".
// The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP
// addresses.
// If the port in the address parameter is empty or "0", as in
// "127.0.0.1:" or "[::1]:0", a port number is automatically chosen.
// The Addr method of Listener can be used to discover the chosen
// port.
func StartServers(servers ...*http.Server) error {
	for idx, s := range servers {
		var err error
		var listener net.Listener
		if hasGracefulRestart {
			listener, err = getListener(strconv.FormatInt(int64(idx), 10))
		} else {
			listener, err = newListener("tcp", strconv.FormatInt(int64(idx), 10), s)
		}
		if err != nil {
			return err
		}

		if hasGracefulRestart {
			go func(server *http.Server, listener net.Listener) { startHttpServer(s, listener) }(s, listener)
			return nil
		}
	}

	for {
		if canStop() {
			break
		}
		select {
		case <-restartChn:
			go startDaemon()
		}
	}
	waitGroup.Wait()

	return nil
}

// Stop stop service. exit process
func Stop() {
	lock.Lock()
	defer lock.Unlock()
	if isStop {
		return
	}
	close(stopChn)
	isStop = true
	waitGroup.Wait()
}

// Restart restart service.
func Restart() {
	// master goroutine not exit.
	// slave  goroutine exit.
	// slave exit. master auto execute new service
	if hasGracefulRestart {
		Stop()
	}
}

// startDaemon
func startDaemon() {
	waitGroup.Add(1)
	defer waitGroup.Done()
	initEnv()
	for {

		extraFiles := make([]*os.File, nextShareFDIdx)
		var envSelf []string
		for _, fdInfoItem := range shareFDs {
			extraFiles[fdInfoItem.idx] = fdInfoItem.fd
			// envItemType := getEnvFDTypePrefix(fdInfoItem.name) // type not set
			envItemIdxKey := getEnvFDIdxPrefix(fdInfoItem.name)
			envSelf = append(envSelf, envItemIdxKey+strconv.FormatInt(int64(fdInfoItem.idx), 10))

		}
		execCmd := exec.Command("/proc/self/exe", args...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.ExtraFiles = extraFiles
		execCmd.Env = append(envSys, envSelf...)
		err := execCmd.Run()
		if err != nil {
			log.Fatalf(err.Error())
		}
		if canStop() {
			return
		}

	}
}

func startHttpServer(server *http.Server, listener net.Listener) {
	exitChn := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		// http.ErrServerClosed 是有http shutdown 引起
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen %s error. err:%s\n", server.Addr, err)
			exitChn <- err
		}
	}()

	waitGroup.Add(1)
	go func() {
		// 等待收到stop 或者http.Serve error 然后退出
		select {
		case <-stopChn:
			server.SetKeepAlivesEnabled(false)
			err := server.Shutdown(context.Background())
			if err != nil {
				log.Fatalf("Could not gracefully shutdown the server: %v \n", err)
			}
		case <-exitChn:
		}

		waitGroup.Done()
	}()
}

// setListener announces on the local network address.
//
// The network must be "tcp", "tcp4", "tcp6".
//  "unix", "unixpacket", "udp", "udp4", "udp6", "unixgram" unimplement
//
// For TCP networks, if the host in the address parameter is empty or
// a literal unspecified IP address, Listen listens on all available
// unicast and anycast IP addresses of the local system.
// To only use IPv4, use network "tcp4".
// The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP
// addresses.
// If the port in the address parameter is empty or "0", as in
// "127.0.0.1:" or "[::1]:0", a port number is automatically chosen.
// The Addr method of Listener can be used to discover the chosen
// port.
//
// See func Dial for a description of the network and address
// parameters.
func newListener(name, network string, server *http.Server) (net.Listener, error) {
	if _, exists := shareFDs[name]; exists {
		return nil, errors.New(name + " duplicate")
	}
	listen, err := net.Listen(network, server.Addr)
	if err != nil {
		return nil, err
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.New(network + " unimplement")
	}

	l := listen.(*net.TCPListener)
	listenFile, err := l.File()
	if err != nil {
		return nil, err
	}
	// 顺序写入，没有锁，不能并发操作
	shareFDs[name] = fdInfo{name: name, idx: getAndSetNextShareFDIdx(), fd: listenFile}

	return listen, nil
}

// getListener retrun net.Listener from delivered file descriptor name
func getListener(name string) (net.Listener, error) {

	idx, err := findFDIndexByName(name)
	if err != nil {
		return nil, err
	}

	/* strType, err := GetFDType(name)
	if err != nil {
		return nil, err
	}

	switch strType {
	case FDTypeSocket.String(), FDTypeHTTP.String():
		//types  can be converted
	default:
		return nil, errors.New(name + " type is " + strType + ". cannot convert to net.Listener")
	} */

	file := os.NewFile(idx, "")
	listenFile, err := net.FileListener(file)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	// after restart, service needs to set share fd
	if _, exists := shareFDs[name]; !exists {
		shareFDs[name] = fdInfo{name: name, idx: getAndSetNextShareFDIdx(), fd: file}
	}
	return listenFile, nil
}

/* // GetFDType get the resource type of the file corresponding to name
func GetFDType(name string) (string, error) {
	key := getEnvFDTypePrefix(name)
	strTypeVal := ""
	for _, envItem := range envSelf {
		if strings.HasSuffix(envItem, key) {
			strTypeVal = strings.TrimPrefix(envItem, key)
		}
	}
	if strTypeVal == "" {
		return "", errors.New(name + " not found")
	}
	return strTypeVal, nil
}
*/

// findFDIndexByName find the fd index of the service by name
func findFDIndexByName(name string) (uintptr, error) {
	key := getEnvFDIdxPrefix(name)
	strIdxVal := ""
	for _, envItem := range envSelf {
		if strings.HasSuffix(envItem, key) {
			strIdxVal = strings.TrimPrefix(envItem, key)
		}
	}
	if strIdxVal == "" {
		return 0, errors.New(name + " not found")
	}
	idx, err := strconv.ParseInt(strIdxVal, 10, 64)
	if err != nil {
		return 0, errors.New(name + " file descriptor index not integer. val: " + strIdxVal + " err: %s" + err.Error())
	}
	// graceful restart delivered fd start no is 3.
	return uintptr((idx) + 3), nil
}
