package iotmaker_docker_builder_demo

import (
	"github.com/hashicorp/memberlist"
	"github.com/helmutkemper/util"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	//kSyncBetweenPodsInterval
	//
	// English:
	//
	// Synchronism interval between instances.
	// The interval serves to prevent the flow of data between instances occupying the network and
	// preventing the communication of other services.
	//
	// Português:
	//
	// Intervalo de sincronismo entre instâncias.
	// O intervalo serve para evitar que o fluxo de dados entre instâncias ocupe a rede e impeça a
	// comunicação dos demais serviços.
	kSyncBetweenPodsInterval = time.Second * 1
)

type Server struct {
	thisInstanceIsReady        bool
	thisNodeAddress            string
	syncPort                   int
	serviceNameList            []string
	memberList                 *memberlist.Memberlist
	syncBetweenInstancesTicker *time.Ticker
	nodeNamesList              *sync.Map
}

// AddServersByName
//
// English:
//
//  Adds a new instance by the name of the service/container.
//
//
// Português:
//
//  Adiciona uma nova instância pelo nome do container/serviço.
func (e *Server) AddServersByName(servers ...string) {
	e.serviceNameList = append(e.serviceNameList, servers...)
}

// ipAddressClear
//
// English:
//
//  Returns only the IPV4 address
//
//  The IP address is given in X.X.X.X.X:PORT format and this function only returns the X.X.X.X address
//
//   Output:
//     address: IPV4 address in X.X.X.X format
//
// Português:
//
//  Retorna apenas o endereço IPV4
//
//  O endereço IP é dado no formato X.X.X.X:PORT e esta função retorna apenas o endereço X.X.X.X
//
//   Saída:
//     address: endereço IPV4 no formato X.X.X.X
func (e *Server) ipAddressClear(address string) (IP string) {
	if strings.Split(address, ":")[0] == "" {
		return
	}
	IP = strings.Split(address, ":")[0]
	return
}

// getAndUpdateThisInstanceAddress
//
// English:
//
//  Updates the current address of the container
//
//  The server address may change when the container is restarted
//
//   Output:
//     IP: Container IPV4 address
//     ready: true if the container is ready to receive requests
//
// Português:
//
//  Atualiza o endereço atual do container
//
//  O endereço do servidor pode mudar quando o container é reiniciado
//
//   Saída:
//     IP: endereço IPV4 do container
//     ready: true se o container está pronto para receber requisições
func (e *Server) getAndUpdateThisInstanceAddress() (IP string, ready bool) {
	var thisNode = e.memberList.LocalNode()
	IP = e.ipAddressClear(thisNode.Addr.String())
	ready = true

	return
}

func (e *Server) Init(syncPort int, servicesListNames ...string) (err error) {
	var ipAddress string

	e.syncPort = syncPort
	e.AddServersByName(servicesListNames...)

	// inicializa a lista de PODs no service discover
	e.memberList, err = memberlist.Create(memberlist.DefaultLANConfig())
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
