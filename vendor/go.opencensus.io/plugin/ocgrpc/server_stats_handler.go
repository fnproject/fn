// Copyright 2017, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package ocgrpc

import (
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"

	ocstats "go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// TagRPC gets the metadata from gRPC context, extracts the encoded tags from
// it and creates a new tag.Map and puts them into the returned context.
func (h *ServerHandler) statsTagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	startTime := time.Now()
	if info == nil {
		if grpclog.V(2) {
			grpclog.Infof("serverHandler.TagRPC called with nil info.", info.FullMethodName)
		}
		return ctx
	}
	d := &rpcData{startTime: startTime}
	ctx, _ = h.createTags(ctx, info.FullMethodName)
	ocstats.Record(ctx, ServerStartedCount.M(1))
	return context.WithValue(ctx, grpcServerRPCKey, d)
}

// HandleRPC processes the RPC events.
func (h *ServerHandler) statsHandleRPC(ctx context.Context, s stats.RPCStats) {
	switch st := s.(type) {
	case *stats.Begin, *stats.InHeader, *stats.InTrailer, *stats.OutHeader, *stats.OutTrailer:
		// Do nothing for server
	case *stats.InPayload:
		h.handleRPCInPayload(ctx, st)
	case *stats.OutPayload:
		// For stream it can be called multiple times per RPC.
		h.handleRPCOutPayload(ctx, st)
	case *stats.End:
		h.handleRPCEnd(ctx, st)
	default:
		grpclog.Infof("unexpected stats: %T", st)
	}
}

func (h *ServerHandler) handleRPCInPayload(ctx context.Context, s *stats.InPayload) {
	d, ok := ctx.Value(grpcServerRPCKey).(*rpcData)
	if !ok {
		if grpclog.V(2) {
			grpclog.Infoln("serverHandler.handleRPCInPayload failed to retrieve *rpcData from context")
		}
		return
	}

	ocstats.Record(ctx, ServerRequestBytes.M(int64(s.Length)))
	atomic.AddInt64(&d.reqCount, 1)
}

func (h *ServerHandler) handleRPCOutPayload(ctx context.Context, s *stats.OutPayload) {
	d, ok := ctx.Value(grpcServerRPCKey).(*rpcData)
	if !ok {
		if grpclog.V(2) {
			grpclog.Infoln("serverHandler.handleRPCOutPayload failed to retrieve *rpcData from context")
		}
		return
	}

	ocstats.Record(ctx, ServerResponseBytes.M(int64(s.Length)))
	atomic.AddInt64(&d.respCount, 1)
}

func (h *ServerHandler) handleRPCEnd(ctx context.Context, s *stats.End) {
	d, ok := ctx.Value(grpcServerRPCKey).(*rpcData)
	if !ok {
		if grpclog.V(2) {
			grpclog.Infoln("serverHandler.handleRPCEnd failed to retrieve *rpcData from context")
		}
		return
	}

	elapsedTime := time.Since(d.startTime)
	reqCount := atomic.LoadInt64(&d.reqCount)
	respCount := atomic.LoadInt64(&d.respCount)

	m := []ocstats.Measurement{
		ServerRequestCount.M(reqCount),
		ServerResponseCount.M(respCount),
		ServerFinishedCount.M(1),
		ServerServerElapsedTime.M(float64(elapsedTime) / float64(time.Millisecond)),
	}

	if s.Error != nil {
		s, ok := status.FromError(s.Error)
		if ok {
			ctx, _ = tag.New(ctx,
				tag.Upsert(KeyStatus, s.Code().String()),
			)
		}
		m = append(m, ServerErrorCount.M(1))
	}

	ocstats.Record(ctx, m...)
}

// createTags creates a new tag map containing the tags extracted from the
// gRPC metadata.
func (h *ServerHandler) createTags(ctx context.Context, fullinfo string) (context.Context, error) {
	mods := []tag.Mutator{
		tag.Upsert(KeyMethod, methodName(fullinfo)),
	}
	if tagsBin := stats.Tags(ctx); tagsBin != nil {
		old, err := tag.Decode([]byte(tagsBin))
		if err != nil {
			return nil, fmt.Errorf("serverHandler.createTags failed to decode tagsBin %v: %v", tagsBin, err)
		}
		return tag.New(tag.NewContext(ctx, old), mods...)
	}
	return tag.New(ctx, mods...)
}
