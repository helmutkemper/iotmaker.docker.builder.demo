package main

import (
	"fmt"
	"github.com/hashicorp/logutils"
	demo "github.com/helmutkemper/iotmaker.docker.builder.demo"
	"github.com/helmutkemper/util"
	"log"
	"os"
	"sync"
	"time"
)

func main() {
	var err error
	var counter = 0.0

	var server = &demo.Server{}
	err = server.Init(1010, "delete_after_test_instance_0")
	if err != nil {
		log.Printf("error: %v", err)
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("WARN"),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		log.Println("chaos enable")
	}()

	timer2 := time.NewTimer(10 * time.Second)
	go func() {
		<-timer2.C
		log.Println("restart-me!")
	}()

	timer3 := time.NewTimer(60 * time.Second)
	go func() {
		<-timer3.C
		util.TraceToLog()
		log.Println("bug: o código não poderia ter entrado aqui")
	}()

	ticker := time.NewTicker(1000 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Printf("counter: %v\n", counter)
				counter += 1.0
			}
		}
	}()

	var wg = &sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
