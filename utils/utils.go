package utils

import (
	"github.com/golang/glog"
	"log"
)

func Must(err error) {
	if err != nil {
		log.Fatalf("aborting due to: %s", err)
	}
}

func MustG(err error) {
	if err != nil {
		glog.Fatalf("aborting due to: %s", err)
	}
}

func Maybe(err error) {
	if err != nil {
		glog.Errorf("%s", err)
	}
}
