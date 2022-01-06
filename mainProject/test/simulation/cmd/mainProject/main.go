package main

import (
	demo "github.com/helmutkemper/iotmaker.docker.builder.demo"
	"github.com/helmutkemper/iotmaker.docker.builder.demo/mainProject/factory"
)

func main() {
	var err error
	var server = &demo.Server{}
	err = server.Init(factory.NewMemberListDefaultConfig(), 1010, "delete_after_test_instance_0")
	if err != nil {
		panic(err)
	}
}
