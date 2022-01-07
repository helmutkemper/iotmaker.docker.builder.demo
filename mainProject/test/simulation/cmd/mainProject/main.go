package main

import (
	demo "github.com/helmutkemper/iotmaker.docker.builder.demo"
	"log"
	"sync"
)

func main() {
	var err error
	var server = &demo.Server{}
	err = server.Init(1010, "delete_after_test_instance_0")
	if err != nil {
		log.Printf("error: %v", err)
	}

	var wg = &sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
