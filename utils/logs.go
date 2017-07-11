package utils

import (
	"flag"
	"github.com/golang/glog"
	"os"
	"strconv"
	"strings"
)

func SetupGlog(verbosity int) {
	os.Args = os.Args[:1]

	flag.Set("v", strconv.Itoa(verbosity))
	flag.Set("logtostderr", "true")
	flag.Parse()
	if verbosity < 5 {
		glog.V(1).Infof("verbosity: %s", strings.Repeat("v", verbosity))
	} else {
		glog.V(1).Infof("verbosity: debug (vvvvv)")
	}
}
