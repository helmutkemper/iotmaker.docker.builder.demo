package main

import (
	"github.com/hashicorp/memberlist"
	demo "github.com/helmutkemper/iotmaker.docker.builder.demo"
	"io/ioutil"
	"time"
)

func NewMemberListDefaultConfig() (conf *memberlist.Config) {
	conf = memberlist.DefaultLANConfig()
	conf.TCPTimeout = time.Second
	conf.IndirectChecks = 1
	conf.RetransmitMult = 2
	conf.SuspicionMult = 3
	conf.PushPullInterval = 15 * time.Second
	conf.ProbeTimeout = 200 * time.Millisecond
	conf.ProbeInterval = time.Second
	conf.GossipInterval = 100 * time.Millisecond
	conf.GossipToTheDeadTime = 15 * time.Second
	conf.LogOutput = ioutil.Discard
	conf.Logger = nil

	return conf
}

func main() {
	var err error
	var server = &demo.Server{}
	err = server.Init(NewMemberListDefaultConfig(), 1010, "delete_after_test_instance_0")
	if err != nil {
		panic(err)
	}
}
