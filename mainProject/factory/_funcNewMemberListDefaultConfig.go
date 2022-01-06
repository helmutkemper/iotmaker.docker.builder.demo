package factory

import (
	"github.com/hashicorp/memberlist"
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
