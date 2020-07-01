package main

import (
	"fmt"
	fPb "github.com/c12s/scheme/flusher"
	hc "github.com/c12s/star/healthcheck"
	"github.com/c12s/star/syncer"
	actor "github.com/c12s/starsystem"
	"github.com/golang/protobuf/proto"
	"strings"
)

type StarAgent struct {
	Conf       *Config
	f          syncer.Syncer
	system     *actor.System
	hcc        hc.Healthchecker
	pointers   []string //for faster lookup
	configPath string
}

func NewStar(c *Config, f syncer.Syncer, hcc hc.Healthchecker, path string) *StarAgent {
	return &StarAgent{
		Conf:       c,
		f:          f,
		hcc:        hcc,
		configPath: path,
	}
}

func (s *StarAgent) addActors(actors map[string]actor.Actor) {
	for key, a := range actors {
		ac := s.system.ActorOf(key, a)
		s.pointers = append(s.pointers, ac.Name())
	}
}

func (s *StarAgent) contains(e string) string {
	for _, a := range s.pointers {
		parts := strings.Split(e, ":")
		if strings.Contains(a, parts[len(parts)-1]) {
			return a
		}
	}
	return ""
}

func (s *StarAgent) Start(actors map[string]actor.Actor) {
	s.system = actor.NewSystem("c12s")
	s.addActors(actors)
	s.sub(s.Conf.RTopic)
}

func (s *StarAgent) sub(topic string) {
	s.f.Subscribe(topic, func(event []byte) {
		data := &fPb.Event{}
		err := proto.Unmarshal(event, data)
		if err == nil {
			key := s.contains(data.Kind)
			if key != "" {
				a := s.system.ActorSelection(key)
				a.Tell(StarMessage{Data: data})
			} else {
				fmt.Println("Such actor do not exists")
			}
		}
	})
}

func (s *StarAgent) Alter(newTopic string) {
	err := s.f.Alter()
	if err != nil {
		fmt.Println("ERROR", err.Error())
		return
	}
	s.sub(newTopic)
}

func (s *StarAgent) Stop() {
	s.system.Shutdown()
}
