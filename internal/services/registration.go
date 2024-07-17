package services

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	magnetarapi "github.com/c12s/magnetar/pkg/api"
	"github.com/c12s/star/internal/domain"
)

type RegistrationService struct {
	client     *magnetarapi.RegistrationAsyncClient
	nodeIdRepo domain.NodeIdStore
}

func NewRegistrationService(client *magnetarapi.RegistrationAsyncClient, nodeIdRepo domain.NodeIdStore) *RegistrationService {
	return &RegistrationService{
		client:     client,
		nodeIdRepo: nodeIdRepo,
	}
}

func (rs *RegistrationService) Register(maxRetries int8, bindAddress string) error {
	req := rs.buildReq(bindAddress)
	for attemptsLeft := maxRetries; attemptsLeft > 0; attemptsLeft-- {
		errChan := make(chan error)
		err := rs.tryRegister(req, errChan)
		if err == nil {
			err = <-errChan
			if err == nil {
				return nil
			}
		}
		log.Println(err)
	}
	return errors.New("max registration attempts exceeded")
}

func (rs *RegistrationService) tryRegister(req *magnetarapi.RegistrationReq, errChan chan<- error) error {
	err := rs.client.Register(req, func(resp *magnetarapi.RegistrationResp) {
		errChan <- rs.nodeIdRepo.Put(domain.NodeId{Value: resp.NodeId})
	})
	return err
}

func (rs *RegistrationService) buildReq(bindAddress string) *magnetarapi.RegistrationReq {
	builder := magnetarapi.NewRegistrationReqBuilder()
	cpuCores, err := cpuCores()
	if err == nil {
		builder = builder.AddFloat64Label("cpu-cores", cpuCores)
	}
	for coreId := 0; coreId < int(cpuCores); coreId++ {
		mhz, err := coreMhz(strconv.Itoa(coreId))
		if err == nil {
			builder = builder.AddFloat64Label(fmt.Sprintf("core%dmhz", coreId), mhz)
		}
		vendorId, err := coreVendorId(strconv.Itoa(coreId))
		if err == nil {
			builder = builder.AddStringLabel(fmt.Sprintf("core%dvendorId", coreId), vendorId)
		}
		model, err := coreModel(strconv.Itoa(coreId))
		if err == nil {
			builder = builder.AddStringLabel(fmt.Sprintf("core%dmodel", coreId), model)
		}
		cacheKB, err := coreCacheKB(strconv.Itoa(coreId))
		if err == nil {
			builder = builder.AddFloat64Label(fmt.Sprintf("core%dcacheKB", coreId), cacheKB)
		}
	}
	fsType, err := fsType()
	if err == nil {
		builder = builder.AddStringLabel("fs-type", fsType)
	}
	diskTotalGB, err := diskTotalGB()
	if err == nil {
		builder = builder.AddFloat64Label("disk-totalGB", diskTotalGB)
	}
	diskFreeGB, err := diskFreeGB()
	if err == nil {
		builder = builder.AddFloat64Label("disk-freeGB", diskFreeGB)
	}
	kernelArch, err := kernelArch()
	if err == nil {
		builder = builder.AddStringLabel("kernel-arch", kernelArch)
	}
	kernelVersion, err := kernelVersion()
	if err == nil {
		builder = builder.AddStringLabel("kernel-version", kernelVersion)
	}
	platform, err := platform()
	if err == nil {
		builder = builder.AddStringLabel("platform", platform)
	}
	platformFamily, err := platformFamily()
	if err == nil {
		builder = builder.AddStringLabel("platform-family", platformFamily)
	}
	platformVersion, err := platformVersion()
	if err == nil {
		builder = builder.AddStringLabel("platform-version", platformVersion)
	}
	memoryTotalGB, err := memoryTotalGB()
	if err == nil {
		builder = builder.AddFloat64Label("memory-totalGB", memoryTotalGB)
	}
	req := builder.Request()
	req.Resources["mem"] = memoryTotalGB
	req.Resources["cpu"] = cpuCores
	req.Resources["disk"] = diskFreeGB
	req.BindAddress = bindAddress
	return req
}

func (rs *RegistrationService) Registered() bool {
	if _, err := rs.nodeIdRepo.Get(); err != nil {
		return false
	}
	return true
}
