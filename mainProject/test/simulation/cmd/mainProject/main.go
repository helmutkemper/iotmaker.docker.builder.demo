package main

import (
	"github.com/hashicorp/logutils"
	demo "github.com/helmutkemper/iotmaker.docker.builder.demo"
	"log"
	"os"
	"sync"
	"time"
)

func main() {
	var err error

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"[DEBUG]", "[WARN]", "[ERROR]"},
		MinLevel: logutils.LogLevel("[WARN]"),
		Writer:   os.Stdout,
	}
	log.SetOutput(filter)

	var server = &demo.Server{}
	err = server.Init(1010, "delete_after_test_instance_0")
	if err != nil {
		log.Printf("error: %v", err)
	}

	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		log.Println("chaos enable")
	}()

	var wg = &sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
