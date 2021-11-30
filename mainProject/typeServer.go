package main

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/helmutkemper/util"
	"google.golang.org/grpc"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	//kSyncBetweenPodsInterval
	//
	// English: Synchronism interval between instances.
	// The break occurs to prevent the flow of data between instances occupying the network and preventing the communication of other services.
	//
	// Português: Intervalo de sincronismo entre instâncias.
	// O intervalo ocorre para evitar que o fluxo de dados entre instâncias ocupe a rede e impeça a comunicação dos demais serviços.
	kSyncBetweenPodsInterval = time.Second * 1
)

type Server struct {
	thisInstanceIsReady        bool
	thisNodeAddress            string
	syncPort                   string
	serviceNameList            []string
	memberList                 *memberlist.Memberlist
	syncBetweenInstancesTicker *time.Ticker
	nodeNamesList              *sync.Map
}

func (e *Server) AddServersByName(servers ...string) {
	e.serviceNameList = append(e.serviceNameList, servers...)
}

func (e *Server) ServicesDiscoverDefaultLocalConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()
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

func (e *Server) ipAddressClear(address string) (IP string) {
	if strings.Split(address, ":")[0] == "" {
		return
	}
	IP = strings.Split(address, ":")[0]
	return
}

func (e *Server) getAndUpdateThisInstanceAddress() (IP string, ready bool) {
	var thisNode = e.memberList.LocalNode()
	IP = e.ipAddressClear(thisNode.Addr.String())
	ready = true

	return
}

func (e *Server) Init(syncPort string, config *memberlist.Config, servicesListNames ...string) (err error) {
	var ipAddress string

	e.syncPort = syncPort
	e.AddServersByName(servicesListNames...)

	if config == nil {
		config = e.ServicesDiscoverDefaultLocalConfig()
	}

	// inicializa a lista de PODs no service discover
	e.memberList, err = memberlist.Create(config)
	if err != nil {
		util.TraceToLog()
		return
	}

	e.nodeNamesList = new(sync.Map)

	// inicializa o ciclo de troca de dados entre pods
	e.syncBetweenInstancesTicker = time.NewTicker(kSyncBetweenPodsInterval)

	err = e.DnsVerifyServices()
	if err != nil {
		util.TraceToLog()
		return
	}

	go func(e *Server) {
		defer func() {
			log.Printf("bug: server aborted")
		}()
		for {
			select {
			case <-e.syncBetweenInstancesTicker.C:

				err = e.DnsVerifyServices()
				if err != nil {
					log.Printf("ticker.verifyDnsForServices().error: %v", err)
					err = nil
				}

				ipAddress, e.thisInstanceIsReady = e.getAndUpdateThisInstanceAddress()
				if e.thisInstanceIsReady == false {
					util.TraceToLog()
					log.Printf("e.getAndUpdateThisInstanceAddress(): recusou")
					continue
				}

				e.thisNodeAddress = ipAddress
			}
		}
	}(e)

	return
}

func (e *Server) DnsVerifyServices() (err error) {
	var pass = false
	var ipServiceList []net.IP
	var ipServiceListAsString = make([]string, 0)

	for _, serviceName := range e.serviceNameList {
		ipServiceList, err = net.LookupIP(serviceName)
		if err == nil {
			pass = true
		}

		for _, nodeIP := range ipServiceList {
			ipServiceListAsString = append(ipServiceListAsString, nodeIP.String())
		}
	}

	if pass == false {
		log.Print("nenhum servidor encontrado")
		return
	}

	if len(ipServiceListAsString) == 0 {
		return
	}

	_, err = e.memberList.Join(ipServiceListAsString)
	if err != nil {
		log.Printf("e.memberList.Join().error: %v", err)
		err = nil
		return
	}

	return
}

func (e *Server) serverGrpc(ip string) (err error) {
	var lis net.Listener
	time.Sleep(2 * time.Second)
	lis, err = net.Listen("tcp", fmt.Sprintf("%v:%d", ip, e.syncPort))
	if err != nil {
		log.Printf("serverGrpc().net.Listen().error: %v", err)
		return
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	grpcProto.RegisterSyncCacheDaraServerServer(grpcServer, e)
	err = grpcServer.Serve(lis)
	if err != nil {
		log.Printf("serverGrpc().grpcServer.Serve().error: %v", err)
		return
	}

	return
}
