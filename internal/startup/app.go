package startup

import (
	"errors"
	"log"
	"net"

	kuiperapi "github.com/c12s/kuiper/pkg/api"
	magnetarapi "github.com/c12s/magnetar/pkg/api"
	meridianapi "github.com/c12s/meridian/pkg/api"
	"github.com/c12s/star/internal/configs"
	"github.com/c12s/star/internal/servers"
	"github.com/c12s/star/internal/services"
	"github.com/c12s/star/internal/store"
	"github.com/c12s/star/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type app struct {
	config               *configs.Config
	grpcServer           *grpc.Server
	configAsyncServer    *servers.ConfigAsyncServer
	appConfigAsyncServer *servers.AppConfigAsyncServer
	shutdownProcesses    []func()
	serfAgent            *services.SerfAgent
	clusterJoinListener  *services.ClusterJoinListener
}

func NewAppWithConfig(config *configs.Config) (*app, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	return &app{
		config:            config,
		shutdownProcesses: make([]func(), 0),
	}, nil
}

func (a *app) init() {
	natsConn, err := NewNatsConn(a.config.NatsAddress())
	if err != nil {
		log.Fatalln(err)
	}
	a.shutdownProcesses = append(a.shutdownProcesses, func() {
		log.Println("closing nats conn")
		natsConn.Close()
	})

	nodeIdStore, err := store.NewNodeIdFSStore(a.config.NodeIdDirPath(), a.config.NodeIdFileName())
	if err != nil {
		log.Fatalln(err)
	}

	registrationClient, err := magnetarapi.NewRegistrationAsyncClient(a.config.NatsAddress())
	if err != nil {
		log.Fatalln(err)
	}

	registrationService := services.NewRegistrationService(registrationClient, nodeIdStore)
	if !registrationService.Registered() {
		err := registrationService.Register(a.config.MaxRegistrationRetries(), a.config.SerfBindAddress())
		if err != nil {
			log.Fatalln(err)
		}
	}

	nodeId, err := nodeIdStore.Get()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("NODE ID: " + nodeId.Value)

	configStore, err := store.NewConfigInMemStore()
	if err != nil {
		log.Fatalln(err)
	}

	agent, err := services.NewSerfAgent(a.config, natsConn, nodeId.Value, configStore)
	if err != nil {
		log.Fatalln(err)
	}
	a.serfAgent = agent

	a.clusterJoinListener = services.NewClusterJoinListener(natsConn, a.serfAgent, nodeId.Value, nodeIdStore)

	configClient, err := kuiperapi.NewKuiperAsyncClient(a.config.NatsAddress(), nodeId.Value)
	if err != nil {
		log.Fatalln(err)
	}
	configAsyncServer, err := servers.NewConfigAsyncServer(configClient, configStore, agent, nodeId.Value)
	if err != nil {
		log.Fatalln(err)
	}
	a.configAsyncServer = configAsyncServer

	meridianClient, err := meridianapi.NewMeridianAsyncClient(a.config.NatsAddress(), nodeId.Value)
	if err != nil {
		log.Fatalln(err)
	}
	appConfigAsyncServer, err := servers.NewAppConfigAsyncServer(meridianClient, agent, nodeId.Value)
	if err != nil {
		log.Fatalln(err)
	}
	a.appConfigAsyncServer = appConfigAsyncServer

	configGrpcServer, err := servers.NewStarConfigServer(configStore)
	if err != nil {
		log.Fatalln(err)
	}

	s := grpc.NewServer()
	api.RegisterStarConfigServer(s, configGrpcServer)
	reflection.Register(s)
	a.grpcServer = s
}

func (a *app) startSerfAgent() error {
	// todo: join on signal
	// err := a.serfAgent.Join(true)
	// if err != nil {
	// 	return err
	// }
	// a.serfAgent.RunMock()
	//a.serfAgent.RunMock2()
	a.serfAgent.Wg.Add(1)
	go a.serfAgent.Listen()
	// go a.serfAgent.ListenNATS()
	return nil
}

func (a *app) startConfigAsyncServer() error {
	a.configAsyncServer.Serve()
	a.appConfigAsyncServer.Serve()
	return nil
}

func (a *app) startGrpcServer() error {
	lis, err := net.Listen("tcp", a.config.GrpcServerAddress())
	if err != nil {
		return err
	}
	go func() {
		log.Printf("server listening at %v", lis.Addr())
		if err := a.grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	return nil
}

func (a *app) Start() error {
	a.init()

	err := a.startConfigAsyncServer()
	if err != nil {
		return err
	}
	err = a.startGrpcServer()
	if err != nil {
		return err
	}
	err = a.startSerfAgent()
	if err != nil {
		return err
	}
	a.clusterJoinListener.Listen()
	return nil
}

func (a *app) GracefulStop() {
	go a.configAsyncServer.GracefulStop()
	a.grpcServer.GracefulStop()
	a.serfAgent.Leave()
	for _, shudownProcess := range a.shutdownProcesses {
		shudownProcess()
	}
}
