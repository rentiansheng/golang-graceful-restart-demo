package upgrade

import (
	"os"
	"strings"
)

func initFramework() {
	initEnv()
	initArgs()
}

func initEnv() {
	envs := os.Environ()
	for _, env := range envs {
		if strings.HasSuffix(env, envPrefix) {
			envSelf = append(envSelf, env)
		} else {
			envSys = append(envSys, env)
		}
	}
	if len(envSelf) > 0 {
		hasGracefulRestart = true
	}

}

func initArgs() {
	if len(args) > 1 {
		args = os.Args[1:]
	}
}
