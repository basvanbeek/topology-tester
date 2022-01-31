// Copyright (c) Bas van Beek 2022.
// Copyright (c) Tetrate, Inc 2021.
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

// Package skywalking provides primitives for creating and configuring a Skywalking
// tracer for this binary.
package skywalking

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/SkyAPM/go2sky"
	go2SkyHttp "github.com/SkyAPM/go2sky/plugins/http"
	"github.com/SkyAPM/go2sky/reporter"
	"github.com/basvanbeek/topology-tester/pkg"
	"github.com/basvanbeek/topology-tester/pkg/observability"
	"github.com/tetratelabs/multierror"
	"github.com/tetratelabs/run"
	"github.com/tetratelabs/run/pkg/version"
)

// flags
const (
	ReporterEndpoint         = "skywalking-reporter-endpoint"
	LocalServicename         = "skywalking-local-servicename"
	LocalServiceInstanceName = "skywalking-local-serviceinstancename"
	SampleRate               = "skywalking-sample-rate"
)

const (
	// default configuration values
	defaultReporterAddr = "oap-skywalking:1180"
	defaultSampleRate   = 1.0
)

// Service implements run.GroupService
type Service struct {
	Servicename         string
	ServiceInstanceName string
	Address             string
	SampleRate          float64
	go2SkyTracer        *go2sky.Tracer

	Reporter     go2sky.Reporter
	ownsReporter bool
	closer       chan error
}

// static compile time run interfaces validation
var (
	_ run.Config                 = (*Service)(nil)
	_ run.PreRunner              = (*Service)(nil)
	_ run.Service                = (*Service)(nil)
	_ observability.Instrumenter = (*Service)(nil)
)

// Name implements run.Unit.
func (s Service) Name() string {
	return "skywalking"
}

// GroupName implements run.Namer so the Skywalking local endpoint service name
// defaults to the name of the run.Group if not set before calling Group's Run
// or RunConfig.
func (s *Service) GroupName(name string) {
	if s.Servicename == "" {
		s.Servicename = name
	}
}

// FlagSet implements run.Config
func (s *Service) FlagSet() *run.FlagSet {
	// set defaults if needed
	if s.Address == "" {
		s.Address = defaultReporterAddr
	}
	if s.Servicename == "" {
		s.Servicename = path.Base(os.Args[0])
	}

	if s.ServiceInstanceName == "" {
		s.ServiceInstanceName = s.Servicename
	}

	if s.SampleRate < 0 {
		s.SampleRate = 0.0
	} else if s.SampleRate == 0.0 {
		s.SampleRate = defaultSampleRate
	}

	// create our configuration flags
	flags := run.NewFlagSet("Skywalking Tracer Config")

	flags.StringVar(
		&s.Address,
		ReporterEndpoint,
		s.Address,
		`Full address, including URI, of the Skywalking HTTP collector`)
	flags.StringVar(
		&s.Servicename,
		LocalServicename,
		s.Servicename,
		`Local ServiceName to report`)
	flags.StringVar(
		&s.ServiceInstanceName,
		LocalServiceInstanceName,
		s.ServiceInstanceName,
		`Local ServiceInstanceName to report`)
	flags.Float64Var(
		&s.SampleRate,
		SampleRate,
		s.SampleRate,
		`Set the Skywalking sample rate, between never (0.0) and always (1.0), `+
			`smallest increment: 0.01`)

	return flags
}

// Validate implements run.Config
func (s Service) Validate() error {
	var mErr error

	if s.Reporter == nil {
		if _, err := url.Parse(s.Address); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf(pkg.FlagErr, ReporterEndpoint, err))
		}
	}
	if s.Servicename == "" {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, LocalServicename, pkg.ErrRequired))
	}
	if s.ServiceInstanceName == "" {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, LocalServiceInstanceName, pkg.ErrRequired))
	}

	return mErr
}

// PreRun implements run.PreRunner
func (s *Service) PreRun() error {
	var err error

	// configure our sampler
	sampler := go2sky.NewRandomSampler(s.SampleRate)

	rep := s.Reporter
	if rep == nil {
		// we create our own reporter
		s.ownsReporter = true
		if rep, err = reporter.NewGRPCReporter(s.Address, reporter.WithCheckInterval(0)); err != nil {
			return err
		}

	}

	// create our tracer
	s.go2SkyTracer, err = go2sky.NewTracer(s.Servicename, go2sky.WithInstance(s.ServiceInstanceName), go2sky.WithReporter(rep), go2sky.WithCustomSampler(sampler))

	if err != nil {
		if s.ownsReporter {
			// we handle the lifecycle of the reporter internally
			rep.Close()
		}
		return err
	}

	s.Reporter = rep
	s.closer = make(chan error)

	return nil
}

// Serve implements run.GroupService
func (s *Service) Serve() error {
	return <-s.closer
}

// GracefulStop implements run.GroupService
func (s *Service) GracefulStop() {
	close(s.closer)
	if s.ownsReporter {
		// we handle the lifecycle of the reporter internally
		s.Reporter.Close()
	}
}

type traceAdapter struct {
	delegate *go2sky.Tracer
}

type spanAdapter struct {
	delegate go2sky.Span
	ctx      context.Context
}

// Context implements observability.Span
func (s *spanAdapter) Context() context.Context {
	return s.ctx
}

func (s *spanAdapter) TraceID() string {
	return go2sky.TraceID(s.ctx)
}

// SetName implements observability.Span
func (s *spanAdapter) SetName(name string) {
	s.delegate.SetOperationName(name)
}

// Tag implements observability.Span
func (s *spanAdapter) Tag(key string, value string) {
	s.delegate.Tag(go2sky.Tag(key), value)
}

// Finish implements observability.Span
func (s *spanAdapter) Finish() {
	s.delegate.End()
}

// SpanFromContext implements observability.Contexter
func (s Service) SpanFromContext(ctx context.Context) observability.Span {
	span := go2sky.ActiveSpan(ctx)
	return &spanAdapter{span, ctx}
}

// StartSpanFromContext implements observability.Tracer
func (t *traceAdapter) StartSpanFromContext(ctx context.Context, name string) observability.Span {
	span, ctx, _ := t.delegate.CreateLocalSpan(ctx, go2sky.WithOperationName(name))
	return &spanAdapter{span, ctx}
}

// Tracer implements observability.Tracerer
func (s *Service) Tracer() observability.Tracer {
	return &traceAdapter{s.go2SkyTracer}
}

// Middleware implements observability.Middlewareer
func (s *Service) Middleware() func(http.Handler) http.Handler {
	mw, _ := go2SkyHttp.NewServerMiddleware(s.go2SkyTracer, go2SkyHttp.WithServerTag(observability.VersionTag, version.Parse())) // err is not nil only when the provided tracer is nil.
	return func(next http.Handler) http.Handler {
		baggageHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := go2sky.GetCorrelation(r.Context(), observability.BaggageRequestID)
			if reqID == "" {
				reqID = r.Header.Get(observability.BaggageRequestID)
			}
			if span := go2sky.ActiveSpan(r.Context()); span != nil {
				span.Tag(observability.BaggageRequestID, reqID)
			}
			go2sky.PutCorrelation(r.Context(), observability.BaggageRequestID, reqID)
			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
		return mw(baggageHandler)
	}
}

// Transport implements observability.Transporter
func (s *Service) Transport(transport http.RoundTripper) (http.RoundTripper, error) {
	var opts []go2SkyHttp.ClientOption
	if transport != nil {
		// Need to create a new client to extract from it the provided transport in go2SkyHttp.NewClient
		opts = append(opts, go2SkyHttp.WithClient(&http.Client{Transport: transport}))
	}
	client, err := go2SkyHttp.NewClient(s.go2SkyTracer, opts...)
	return client.Transport, err
}
