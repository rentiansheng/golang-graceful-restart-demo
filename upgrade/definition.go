package upgrade

import (
	"os"
	"sync"
)

var (
	// execute service system environment
	envSys []string
	// execute service graceful framework environment
	envSelf []string
	// graceful restart  delivered file descriptor
	shareFDs = make(map[string]fdInfo, 0)
	// execute service args
	args []string
	// execute service directory
	pwd string
	//  whether it has been restarted
	hasGracefulRestart = false
	// next shared fd index, only the first startup setting is allowed
	nextShareFDIdx = 0

	waitGroup sync.WaitGroup

	// want all http.Serve to send a stop signal
	stopChn = make(chan struct{}, 1)
	isStop  = false
	// want all http.Server to send restart
	restartChn = make(chan struct{}, 1)

	lock sync.RWMutex
)

// fdInfo shared fd info
type fdInfo struct {
	name   string
	idx    int
	fd     *os.File
	fdType FDType
}

// FDType file descriptor type
type FDType int

const (
	// FDTypeSocket socket file descriptor
	FDTypeSocket FDType = iota + 1
	// FDTypeHTTP http server file descriptor
	FDTypeHTTP
)

func (fd FDType) String() string {
	switch fd {
	case FDTypeSocket:
		return "socket"
	case FDTypeHTTP:
		return "http"
	}
	return ""
}

const (
	// execute service environment prefix
	envPrefix = "__graceful_framework__"
	// env share fd name flag
	envFDNameFlag = "_type"
	// env share fd idx flag
	envFDIdxFlag = "_idx"
)

func getEnvFDTypePrefix(name string) string {
	return envPrefix + name + "_" + envFDNameFlag + "="
}

func getEnvFDIdxPrefix(name string) string {
	return envPrefix + name + "_" + envFDIdxFlag + "="
}

// getAndSetNextShareFDIdx get the order of the current file in sharing fd, and set next share fd index
func getAndSetNextShareFDIdx() int {
	curIdx := nextShareFDIdx
	// set next idx
	nextShareFDIdx++
	return curIdx
}

func canStop() bool {
	lock.RLock()
	defer lock.RUnlock()
	return isStop
}
