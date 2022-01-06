package main

import (
	demo "github.com/helmutkemper/iotmaker.docker.builder.demo"
	"io/ioutil"
	"time"
)

func main() {
	var err error
	var server = &demo.Server{}
	err = server.Init(1010, "delete_after_test_instance_0")
	if err != nil {
		panic(err)
	}
}
