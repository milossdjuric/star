package main

import (
	"fmt"
	fPb "github.com/c12s/scheme/flusher"
	sPb "github.com/c12s/scheme/stellar"
	hcc "github.com/c12s/star/healthcheck"
	"github.com/c12s/star/syncer"
	actor "github.com/c12s/starsystem"
	sg "github.com/c12s/stellar-go"
	"strings"
)

//
// helper
//
func parse(tags string) map[string]string {
	rez := map[string]string{}
	for _, item := range strings.Split(tags, ";") {
		pair := strings.Split(item, ":")
		rez[pair[0]] = pair[1]
	}

	return rez
}

func syncTopic(id string) string {
	return strings.Join([]string{id, ">"}, ".")
}

//
// Star Message
//
type StarMessage struct {
	Data *fPb.Event
}

func (m StarMessage) Name() string {
	return "StarMessage"
}

func (m StarMessage) Params() map[string][]byte {
	return nil
}

//
// Configs Actor
//
type ConfigsActor struct {
	uploader syncer.Uploader
}

func (m ConfigsActor) Receive(msg interface{}, context *actor.ActorProp) {
	switch data := msg.(type) {
	case StarMessage:
		fmt.Println("Received Configs")
		fmt.Println(data)
		span, _ := sg.FromCustomSource(
			data.Data.SpanContext,
			data.Data.SpanContext.Baggage,
			"actor.configs",
		)
		fmt.Println(span)
		defer span.Finish()

		ssp := span.Serialize()
		m.uploader.Upload(&fPb.Update{
			TaskKey: data.Data.TaskKey,
			Kind:    data.Data.Kind,
			Node:    m.uploader.NodeId(),
			SpanContext: &sPb.SpanContext{
				TraceId:       ssp.Get("trace_id")[0],
				SpanId:        ssp.Get("span_id")[0],
				ParrentSpanId: ssp.Get("parrent_span_id")[0],
				Baggage:       parse(ssp.Get("tags")[0]),
			},
		})
	default:
		fmt.Println("Error")
	}
}

//
// Actions Actor
//
type ActionsActor struct {
	uploader syncer.Uploader
}

func (m ActionsActor) Receive(msg interface{}, context *actor.ActorProp) {
	switch data := msg.(type) {
	case StarMessage:
		fmt.Println("Received Actions")
		fmt.Println(data)
		span, _ := sg.FromCustomSource(
			data.Data.SpanContext,
			data.Data.SpanContext.Baggage,
			"actor.actions",
		)
		fmt.Println(span)
		defer span.Finish()

		ssp := span.Serialize()
		m.uploader.Upload(&fPb.Update{
			TaskKey: data.Data.TaskKey,
			Kind:    data.Data.Kind,
			Node:    m.uploader.NodeId(),
			SpanContext: &sPb.SpanContext{
				TraceId:       ssp.Get("trace_id")[0],
				SpanId:        ssp.Get("span_id")[0],
				ParrentSpanId: ssp.Get("parrent_span_id")[0],
				Baggage:       parse(ssp.Get("tags")[0]),
			},
		})
	default:
		fmt.Println("Error")
	}
}

//
// Secrets Actor
//
type SecretsActor struct {
	uploader syncer.Uploader
}

func (m SecretsActor) Receive(msg interface{}, context *actor.ActorProp) {
	switch data := msg.(type) {
	case StarMessage:
		fmt.Println("Received Secrets")
		fmt.Println(data)
		span, _ := sg.FromCustomSource(
			data.Data.SpanContext,
			data.Data.SpanContext.Baggage,
			"actor.secrets",
		)
		fmt.Println(span)
		defer span.Finish()

		ssp := span.Serialize()
		m.uploader.Upload(&fPb.Update{
			TaskKey: data.Data.TaskKey,
			Kind:    data.Data.Kind,
			Node:    m.uploader.NodeId(),
			SpanContext: &sPb.SpanContext{
				TraceId:       ssp.Get("trace_id")[0],
				SpanId:        ssp.Get("span_id")[0],
				ParrentSpanId: ssp.Get("parrent_span_id")[0],
				Baggage:       parse(ssp.Get("tags")[0]),
			},
		})
	default:
		fmt.Println("Error")
	}
}

//
// Topology Actor
//
type TopologyActor struct {
	uploader syncer.Uploader
}

func (m TopologyActor) Receive(msg interface{}, context *actor.ActorProp) {
	switch data := msg.(type) {
	case StarMessage:
		fmt.Println("Received Topology")
		fmt.Println(data)
		span, _ := sg.FromCustomSource(
			data.Data.SpanContext,
			data.Data.SpanContext.Baggage,
			"actor.topology",
		)
		fmt.Println(span)
		defer span.Finish()

		ssp := span.Serialize()
		m.uploader.Upload(&fPb.Update{
			TaskKey: data.Data.TaskKey,
			Kind:    data.Data.Kind,
			Node:    m.uploader.NodeId(),
			SpanContext: &sPb.SpanContext{
				TraceId:       ssp.Get("trace_id")[0],
				SpanId:        ssp.Get("span_id")[0],
				ParrentSpanId: ssp.Get("parrent_span_id")[0],
				Baggage:       parse(ssp.Get("tags")[0]),
			},
		})

		updateActor := context.Context.ActorSelection("c12s/update")
		if updateActor != nil {
			for _, p := range data.Data.Payload {
				if p.Value["ID"] == m.uploader.NodeId() {
					updateActor.Tell(UpdateConfigMessage{
						NewID: p.Value["NEW_ID"],
					})
				}
				continue
			}
		}
	default:
		fmt.Println("Error")
	}
}

//
// UpdateConfig Actor
//
type UpdateActor struct {
	c    *Config
	path string
	hc   hcc.Healthchecker
	s    *StarAgent
}

//
// UpdateConfgMessage
//
type UpdateConfigMessage struct {
	NewID string
}

func (a UpdateActor) Receive(msg interface{}, context *actor.ActorProp) {
	switch data := msg.(type) {
	case UpdateConfigMessage:
		a.hc.UpdateId(data.NewID)
		a.c.UpdateId(data.NewID)
		a.c.UpdateSyncTopic(syncTopic(data.NewID))
		a.c.SaveFile(a.path)
		a.s.Alter(syncTopic(data.NewID))
	default:
		fmt.Println("Error")
	}
}
