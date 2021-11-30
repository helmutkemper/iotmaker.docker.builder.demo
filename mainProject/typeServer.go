package main

import (
	"context"
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/helmutkemper/iotmaker.docker.builder.demo/grpcProto"
	"github.com/helmutkemper/util"
	"google.golang.org/grpc"
	"io/ioutil"
	"log"
	"net"
	"strconv"
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
	grpcProto.UnimplementedSyncInstancesServer

	thisInstanceIsReady        bool
	thisNodeAddress            string
	syncPort                   int
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

func (e *Server) Init(syncPort int, config *memberlist.Config, servicesListNames ...string) (err error) {
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

				e.dataTransmission()
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
	grpcProto.RegisterSyncInstancesServer(grpcServer, e)
	err = grpcServer.Serve(lis)
	if err != nil {
		log.Printf("serverGrpc().grpcServer.Serve().error: %v", err)
		return
	}

	return
}

func (e *Server) dataTransmission() {
	var err error
	var ipListToSync []string
	var nodeShutdownList map[string]string
	var nodeAddedList map[string]string
	ipListToSync, nodeShutdownList, nodeAddedList = e.getMembersListToSyncDataAndManagerSyncList()

	var conn *grpc.ClientConn
	var client grpcProto.SyncInstancesClient

	for _, ipAddress := range nodeAddedList {
		log.Printf("ip address %v has been added", ipAddress)
	}

	for _, ipAddress := range nodeShutdownList {
		log.Printf("ip address %v is shunted down", ipAddress)
	}

	for _, podIpAddress := range ipListToSync {
		conn, client, err = e.clientGrpcOpen(podIpAddress + ":" + strconv.Itoa(e.syncPort))
		if err != nil {
			log.Printf("e.clientGrpcOpen().error: %v", err)
			return
		}

		var ctx context.Context
		var cancelContext context.CancelFunc
		var ready *grpcProto.InstanceIsReadyReplay
		ctx, cancelContext = context.WithTimeout(context.Background(), 500*time.Millisecond)
		ready, err = client.GrpcFuncInstanceIsReady(ctx, &grpcProto.Empty{})
		cancelContext()
		if err != nil {
			e.clientGrpcClose(conn)
			log.Printf("client.GrpcFuncInstanceIsReady().error: %v", err)
			continue
		}

		if ready.GetIsReady() == false {
			continue
		}

		ctx, cancelContext = context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, err = client.GrpcFuncCommunication(ctx, &grpcProto.Empty{})
		cancelContext()
		if err != nil {
			e.clientGrpcClose(conn)
			log.Printf("client.GrpcFuncCommunication().error: %v", err)
			continue
		}
	}

	return
}

func (e *Server) clientGrpcOpen(serverAddr string) (conn *grpc.ClientConn, client grpcProto.SyncInstancesClient, err error) {
	conn, err = grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("grpc.Dial(%v).error: %v", serverAddr, err)
		return
	}

	client = grpcProto.NewSyncInstancesClient(conn)
	return
}

func (e *Server) getMembersListToSyncDataAndManagerSyncList() (ipList []string, nodeShutdownList, nodeAddedList map[string]string) {
	//lista de nodes atual
	ipList = make([]string, 0)

	// pega a lista de nodes desligados
	// chave: nome do node / valor: ip do node
	nodeShutdownList = make(map[string]string)

	// pega a lista de nodes adicionados
	// chave: nome do node / valor: ip do node
	nodeAddedList = make(map[string]string)

	if e.nodeNamesList == nil {
		e.nodeNamesList = new(sync.Map)
	}

	var ipAddress string

	// testa se um node tem nome registrado e um ip diferente do original
	var originalIpAddress interface{}

	// preenche a lista atual de membros
	// chave: nome do node / valor: ip do node
	var nodeActualMembersList = make(map[string]string)
	for _, node := range e.memberList.Members() {
		ipAddress = e.ipAddressClear(node.Address())
		nodeActualMembersList[node.Name] = ipAddress

		ipList = append(ipList, ipAddress)
	}

	// pega a lista de nodes desligados
	// chave: nome do node / valor: ip do node
	var found bool
	e.nodeNamesList.Range(func(nodeName, nodeIpAddress interface{}) (continueLoop bool) {
		_, found = nodeActualMembersList[nodeName.(string)]
		if found == false {
			nodeShutdownList[nodeName.(string)] = nodeIpAddress.(string)
		}

		continueLoop = true
		return
	})

	// pega a lista de nodes adicionados
	// chave: nome do node / valor: ip do node
	for nodeName, nodeIpAddress := range nodeActualMembersList {
		originalIpAddress, found = e.nodeNamesList.Load(nodeName)
		if found == false {
			nodeAddedList[nodeName] = nodeIpAddress
		}

		// adiciona o node a lista permanente de nodes
		e.nodeNamesList.Store(nodeName, nodeIpAddress)

		// procura um bug ou falha de conceito
		if originalIpAddress != nil && originalIpAddress.(string) != nodeActualMembersList[nodeName] {
			log.Printf("bug: o endereço IP do node está variando para o node.name")
		}
	}

	return
}

func (e *Server) clientGrpcClose(conn *grpc.ClientConn) {
	var err error
	err = conn.Close()
	if err != nil {
		log.Printf("clientGrpcClose().error: %v", err)
		return
	}

	return
}

func (e *Server) grpcFuncInstanceIsReady(context.Context, *grpcProto.Empty) (replay *grpcProto.InstanceIsReadyReplay, err error) {
	replay = &grpcProto.InstanceIsReadyReplay{
		IsReady: e.thisInstanceIsReady,
	}

	return
}

func (e *Server) GrpcFuncCommunication(context.Context, *grpcProto.Empty) (empty *grpcProto.Empty, err error) {
	empty = &grpcProto.Empty{}

	return
}
