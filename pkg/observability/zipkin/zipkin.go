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

// Package zipkin provides primitives for creating and configuring a Zipkin
// tracer for this binary.
package zipkin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/basvanbeek/topology-tester/pkg/observability"
	"github.com/openzipkin/zipkin-go"
	zmw "github.com/openzipkin/zipkin-go/middleware/http"
	"github.com/openzipkin/zipkin-go/propagation/baggage"
	"github.com/openzipkin/zipkin-go/reporter"
	zrpr "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/tetratelabs/multierror"
	"github.com/tetratelabs/run"
	"github.com/tetratelabs/run/pkg/version"

	"github.com/basvanbeek/topology-tester/pkg"
)

// flags
const (
	ReporterEndpoint = "zipkin-reporter-endpoint"
	LocalServicename = "zipkin-local-servicename"
	LocalHostport    = "zipkin-local-hostport"
	SinglehostSpans  = "zipkin-singlehost-spans"
	SampleRate       = "zipkin-sample-rate"
)

const (
	// default configuration values
	defaultReporterAddr = "http://zipkin:9411/api/v2/spans"
	defaultSampleRate   = 1.0
)

// Service implements run.GroupService
type Service struct {
	Servicename     string
	LocalHostport   string
	Address         string
	SampleRate      float64
	zipkinTracer    *zipkin.Tracer
	Reporter        reporter.Reporter
	SingleHostSpans bool

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
	return "zipkin"
}

// GroupName implements run.Namer so the Zipkin local endpoint service name
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
	if s.SampleRate < 0 {
		s.SampleRate = 0.0
	} else if s.SampleRate == 0.0 {
		s.SampleRate = defaultSampleRate
	}

	// create our configuration flags
	flags := run.NewFlagSet("Zipkin Tracer Config")

	flags.StringVar(
		&s.Address,
		ReporterEndpoint,
		s.Address,
		`Full address, including URI, of the Zipkin HTTP collector`)
	flags.StringVar(
		&s.Servicename,
		LocalServicename,
		s.Servicename,
		`Local ServiceName to report`)
	flags.StringVar(
		&s.LocalHostport,
		LocalHostport,
		s.LocalHostport,
		`Local ip:port to report`)
	flags.BoolVar(
		&s.SingleHostSpans,
		SinglehostSpans,
		false,
		`Do not use Zipkin RPC shared spans`)
	flags.Float64Var(
		&s.SampleRate,
		SampleRate,
		s.SampleRate,
		`Set the Zipkin sample rate, between never (0.0) and always (1.0), `+
			`smallest increment: 0.0001`)

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
	if s.LocalHostport != "" {
		if _, _, err := net.SplitHostPort(s.LocalHostport); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf(pkg.FlagErr, LocalHostport, err))
		}
	}
	if _, err := zipkin.NewBoundarySampler(s.SampleRate, 0); err != nil {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, SampleRate, err))
	}

	return mErr
}

// PreRun implements run.PreRunner
func (s *Service) PreRun() error {
	var err error

	// configure our local endpoint
	ep, err := zipkin.NewEndpoint(s.Servicename, s.LocalHostport)
	if err != nil {
		return err
	}

	// configure our sampler
	salt := time.Now().UnixNano()
	sampler, err := zipkin.NewBoundarySampler(s.SampleRate, salt)
	if err != nil {
		return err
	}

	rep := s.Reporter
	if rep == nil {
		// we create our own reporter
		s.ownsReporter = true
		rep = zrpr.NewReporter(s.Address)
	}

	// create our tracer
	s.zipkinTracer, err = zipkin.NewTracer(
		rep,
		zipkin.WithLocalEndpoint(ep),
		zipkin.WithSharedSpans(!s.SingleHostSpans),
		zipkin.WithSampler(sampler),
		zipkin.WithTags(map[string]string{observability.VersionTag: version.Parse()}),
	)
	if err != nil {
		if s.ownsReporter {
			// we handle the lifecycle of the reporter internally
			_ = rep.Close() // nolint: errcheck
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
		_ = s.Reporter.Close() // nolint: errcheck
	}
}

type traceAdapter struct {
	delegate *zipkin.Tracer
}

type spanAdapter struct {
	delegate zipkin.Span
	ctx      context.Context
}

// Context implements observability.Span
func (s *spanAdapter) Context() context.Context {
	return s.ctx
}

// TraceID implements observability.Span
func (s *spanAdapter) TraceID() string {
	return s.delegate.Context().TraceID.String()
}

// SetName implements observability.Span
func (s *spanAdapter) SetName(name string) {
	s.delegate.SetName(name)
}

// Tag implements observability.Span
func (s *spanAdapter) Tag(key string, value string) {
	s.delegate.Tag(key, value)
}

// Finish implements observability.Span
func (s *spanAdapter) Finish() {
	s.delegate.Finish()
}

// StartSpanFromContext implements observability.Tracer
func (t *traceAdapter) StartSpanFromContext(ctx context.Context, name string) observability.Span {
	span, ctx := t.delegate.StartSpanFromContext(ctx, name)
	return &spanAdapter{span, ctx}
}

// SpanFromContext implements observability.Contexter
func (s *Service) SpanFromContext(ctx context.Context) observability.Span {
	span := zipkin.SpanOrNoopFromContext(ctx)
	return &spanAdapter{span, ctx}
}

// Tracer implements observability.Tracerer
func (s Service) Tracer() observability.Tracer {
	return &traceAdapter{delegate: s.zipkinTracer}
}

// Middleware implements observability.Middlewareer
func (s *Service) Middleware() func(http.Handler) http.Handler {
	// Add baggage fields to be extracted and propagated.
	baggageHandler := baggage.New(observability.BaggageRequestID)

	return zmw.NewServerMiddleware(s.zipkinTracer, zmw.EnableBaggage(baggageHandler))
}

// Transport implements observability.Transporter
func (s *Service) Transport(transport http.RoundTripper) (http.RoundTripper, error) {
	return zmw.NewTransport(s.zipkinTracer, zmw.RoundTripper(transport))
}
