// Copyright 2014 The mqrouter Author. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mq

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/shawnfeng/sutil/scontext"
	"github.com/shawnfeng/sutil/slog/slog"
	"github.com/shawnfeng/sutil/stime"
	"time"
	// kafka "github.com/segmentio/kafka-go"
)

const (
	spanLogKeyKey            = "key"
	spanLogKeyTopic          = "topic"
	spanLogKeyMQType         = "mq"
	spanLogKeyKafkaGroupID   = "groupid"
	spanLogKeyKafkaPartition = "partition"
	spanLogKeyKafkaBrokers   = "brokers"

	defaultRouteGroup = "default"
)

var mqOpDurationLimit = 10 * time.Millisecond

type Message struct {
	Key   string
	Value interface{}
}

func WriteMsg(ctx context.Context, topic string, key string, value interface{}) error {
	fun := "mq.WriteMsg -->"

	span, ctx := opentracing.StartSpanFromContext(ctx, "mq.WriteMsg")
	defer span.Finish()
	span.LogFields(
		log.String(spanLogKeyTopic, topic),
		log.String(spanLogKeyKey, key))

	conf := &instanceConf{
		group:     scontext.GetControlRouteGroupWithDefault(ctx, defaultRouteGroup),
		role:      RoleTypeWriter,
		topic:     topic,
		groupId:   "",
		partition: 0,
	}
	writer := defaultInstanceManager.getWriter(ctx, conf)
	if writer == nil {
		slog.Errorf(ctx, "%s getWriter err, topic: %s", fun, topic)
		return fmt.Errorf("%s, getWriter err, topic: %s", fun, topic)
	}

	payload, err := generatePayload(ctx, value)
	if err != nil {
		slog.Errorf(ctx, "%s generatePayload err, topic: %s", fun, topic)
		return fmt.Errorf("%s, generatePayload err, topic: %s", fun, topic)
	}

	st := stime.NewTimeStat()
	defer func() {
		dur := st.Duration()
		if dur > mqOpDurationLimit {
			slog.Infof(ctx, "%s slow topic:%s dur:%d", fun, topic, dur)
		}
	}()

	return writer.WriteMsg(ctx, key, payload)
}

func WriteMsgs(ctx context.Context, topic string, msgs ...Message) error {
	fun := "mq.WriteMsgs -->"

	span, ctx := opentracing.StartSpanFromContext(ctx, "mq.WriteMsgs")
	defer span.Finish()
	span.LogFields(log.String(spanLogKeyTopic, topic))

	conf := &instanceConf{
		group:     scontext.GetControlRouteGroupWithDefault(ctx, defaultRouteGroup),
		role:      RoleTypeWriter,
		topic:     topic,
		groupId:   "",
		partition: 0,
	}
	writer := defaultInstanceManager.getWriter(ctx, conf)
	if writer == nil {
		slog.Errorf(ctx, "%s getWriter err, topic: %s", fun, topic)
		return fmt.Errorf("%s, getWriter err, topic: %s", fun, topic)
	}

	nmsgs, err := generateMsgsPayload(ctx, msgs...)
	if err != nil {
		slog.Errorf(ctx, "%s generateMsgsPayload err, topic: %s", fun, topic)
		return fmt.Errorf("%s, generateMsgsPayload err, topic: %s", fun, topic)
	}

	st := stime.NewTimeStat()
	defer func() {
		dur := st.Duration()
		if dur > mqOpDurationLimit {
			slog.Infof(ctx, "%s slow topic:%s dur:%d", fun, topic, dur)
		}
	}()

	return writer.WriteMsgs(ctx, nmsgs...)
}

// 读完消息后会自动提交offset
func ReadMsgByGroup(ctx context.Context, topic, groupId string, value interface{}) (context.Context, error) {
	fun := "mq.ReadMsgByGroup -->"

	span, ctx := opentracing.StartSpanFromContext(ctx, "mq.ReadMsgByGroup")
	defer span.Finish()
	span.LogFields(
		log.String(spanLogKeyTopic, topic))

	conf := &instanceConf{
		group:     scontext.GetControlRouteGroupWithDefault(ctx, defaultRouteGroup),
		role:      RoleTypeReader,
		topic:     topic,
		groupId:   groupId,
		partition: 0,
	}
	reader := defaultInstanceManager.getReader(ctx, conf)
	if reader == nil {
		slog.Errorf(ctx, "%s getReader err, topic: %s", fun, topic)
		return nil, fmt.Errorf("%s, getReader err, topic: %s", fun, topic)
	}

	var payload Payload
	st := stime.NewTimeStat()

	err := reader.ReadMsg(ctx, &payload, value)

	dur := st.Duration()
	if dur > mqOpDurationLimit {
		slog.Infof(ctx, "%s slow topic:%s groupId:%s dur:%d", fun, topic, groupId, dur)
	}

	if err != nil {
		slog.Errorf(ctx, "%s ReadMsg err: %v, topic: %s", fun, err, topic)
		return nil, fmt.Errorf("%s, ReadMsg err: %v, topic: %s", fun, err, topic)
	}

	if len(payload.Value) == 0 {
		return context.TODO(), nil
	}

	mctx, err := parsePayload(&payload, "mq.ReadMsgByGroup", value)
	mspan := opentracing.SpanFromContext(mctx)
	if mspan != nil {
		defer mspan.Finish()
		mspan.LogFields(
			log.String(spanLogKeyTopic, topic))
	}
	return mctx, err
}

//
func ReadMsgByPartition(ctx context.Context, topic string, partition int, value interface{}) (context.Context, error) {
	fun := "mq.ReadMsgByPartition -->"

	span, ctx := opentracing.StartSpanFromContext(ctx, "mq.ReadMsgByPartition")
	defer span.Finish()
	span.LogFields(
		log.String(spanLogKeyTopic, topic))

	conf := &instanceConf{
		group:     scontext.GetControlRouteGroupWithDefault(ctx, defaultRouteGroup),
		role:      RoleTypeReader,
		topic:     topic,
		groupId:   "",
		partition: partition,
	}
	reader := defaultInstanceManager.getReader(ctx, conf)
	if reader == nil {
		slog.Errorf(ctx, "%s getReader err, topic: %s", fun, topic)
		return nil, fmt.Errorf("%s, getReader err, topic: %s", fun, topic)
	}

	var payload Payload
	st := stime.NewTimeStat()

	err := reader.ReadMsg(ctx, &payload, value)

	dur := st.Duration()
	if dur > mqOpDurationLimit {
		slog.Infof(ctx, "%s slow topic:%s partition:%d dur:%d", fun, topic, partition, dur)
	}

	if err != nil {
		slog.Errorf(ctx, "%s ReadMsg err: %v, topic: %s", fun, err, topic)
		return nil, fmt.Errorf("%s, ReadMsg err: %v, topic: %s", fun, err, topic)
	}

	if len(payload.Value) == 0 {
		return context.TODO(), nil
	}

	mctx, err := parsePayload(&payload, "mq.ReadMsgByPartition", value)
	mspan := opentracing.SpanFromContext(mctx)
	if mspan != nil {
		defer mspan.Finish()
		mspan.LogFields(
			log.String(spanLogKeyTopic, topic))
	}
	return mctx, err
}

// 读完消息后不会自动提交offset,需要手动调用Handle.CommitMsg方法来提交offset
func FetchMsgByGroup(ctx context.Context, topic, groupId string, value interface{}) (context.Context, Handler, error) {
	fun := "mq.FetchMsgByGroup -->"

	span, ctx := opentracing.StartSpanFromContext(ctx, "mq.FetchMsgByGroup")
	defer span.Finish()
	span.LogFields(
		log.String(spanLogKeyTopic, topic))

	conf := &instanceConf{
		group:     scontext.GetControlRouteGroupWithDefault(ctx, defaultRouteGroup),
		role:      RoleTypeReader,
		topic:     topic,
		groupId:   groupId,
		partition: 0,
	}
	reader := defaultInstanceManager.getReader(ctx, conf)
	if reader == nil {
		slog.Errorf(ctx, "%s getReader err, topic: %s", fun, topic)
		return nil, nil, fmt.Errorf("%s, getReader err, topic: %s", fun, topic)
	}

	var payload Payload
	st := stime.NewTimeStat()

	handler, err := reader.FetchMsg(ctx, &payload, value)

	dur := st.Duration()
	if dur > mqOpDurationLimit {
		slog.Infof(ctx, "%s slow topic:%s groupId:%s dur:%d", fun, topic, groupId, dur)
	}

	if err != nil {
		slog.Errorf(ctx, "%s ReadMsg err: %v, topic: %s", fun, err, topic)
		return nil, nil, fmt.Errorf("%s, ReadMsg err: %v, topic: %s", fun, err, topic)
	}

	if len(payload.Value) == 0 {
		return context.TODO(), handler, nil
	}

	mctx, err := parsePayload(&payload, "mq.FetchMsgByGroup", value)
	mspan := opentracing.SpanFromContext(mctx)
	if mspan != nil {
		defer mspan.Finish()
		mspan.LogFields(
			log.String(spanLogKeyTopic, topic))
	}
	return mctx, handler, err
}

func SetConfiger(ctx context.Context, configerType ConfigerType) error {
	fun := "mq.SetConfiger-->"
	configer, err := NewConfiger(configerType)
	if err != nil {
		slog.Infof(ctx, "%s set configer:%v err:%v", fun, configerType, err)
		return err
	}
	DefaultConfiger = configer
	return DefaultConfiger.Init(ctx)
}

func WatchUpdate(ctx context.Context) {
	go defaultInstanceManager.watch(ctx)
}

func Close() {
	defaultInstanceManager.Close()
}

func init() {
	fun := "mq.init -->"
	ctx := context.Background()
	err := SetConfiger(ctx, ConfigerTypeApollo)
	if err != nil {
		slog.Errorf(ctx, "%s set mq configer:%v err:%v", fun, ConfigerTypeApollo, err)
	} else {
		slog.Infof(ctx, "%s mq configer:%v been set", fun, ConfigerTypeApollo)
	}
}
