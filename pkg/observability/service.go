package observability

import (
	"context"
	"fmt"
	"net/http"

	"github.com/basvanbeek/topology-tester/pkg"
	"github.com/tetratelabs/multierror"
	"github.com/tetratelabs/run"
)

const (
	ObservabilityInstrumenter = "observability-instrumenter"
	ZipkinInstrumenter        = "zipkin"
	SkywalkingInstrumenter    = "skywalking"

	BaggageRequestID = "X-Request-Id"
	VersionTag       = "version"
)

// Tracerer is an extension interface that observability Services can implement
// to provide tracing functionalities.
type Tracerer interface {
	Tracer() Tracer
}

// Middlewareer is an extension interface that observability Services can implement
// to provide an instrumented middleware.
type Middlewareer interface {
	Middleware() func(http.Handler) http.Handler
}

// Transporter is an extension interface that observability Services can implement
// to provide an instrumented http.RoundTripper.
type Transporter interface {
	Transport(transport http.RoundTripper) (http.RoundTripper, error)
}

// Instrumenter is an interface a concrete tracing provider needs to implement.
type Instrumenter interface {
	Tracerer
	Contexter
	Middlewareer
	Transporter
}

// InstrumenterService is an interface a concrete service tracing provider needs to implement.
type InstrumenterService interface {
	Instrumenter
	run.Config
	run.PreRunner
	run.Service
}

// Service implements run.GroupService
type Service struct {
	ObservabilityInstrumenter string
	Instrumenters             []InstrumenterService

	delegate InstrumenterService
}

// static compile time run interfaces validation
var (
	_ run.Config    = (*Service)(nil)
	_ run.PreRunner = (*Service)(nil)
	_ run.Service   = (*Service)(nil)
	_ Instrumenter  = (*Service)(nil)
)

func supportedInstrumenters() []string {
	return []string{ZipkinInstrumenter, SkywalkingInstrumenter}
}

// Name implements run.Unit.
func (s *Service) Name() string {
	if s.delegate == nil {
		return "observability-instrumenter"
	}
	return fmt.Sprintf("observability-instrumenter[%s]", s.delegate.Name())
}

// FlagSet implements run.Config
func (s *Service) FlagSet() *run.FlagSet {
	// create our configuration flags
	flags := run.NewFlagSet("Observability instrumenter config")

	flags.StringVar(
		&s.ObservabilityInstrumenter,
		ObservabilityInstrumenter,
		s.ObservabilityInstrumenter,
		fmt.Sprintf(`Name of the instrumenter to use, one of %v`, supportedInstrumenters()))

	for _, instrumenter := range s.Instrumenters {
		flags.AddFlagSet(instrumenter.FlagSet().FlagSet)
	}
	return flags
}

// Validate implements run.Config
func (s *Service) Validate() error {
	var mErr error

	var foundSupportedInstrumenter bool
	for _, name := range supportedInstrumenters() {
		if name == s.ObservabilityInstrumenter {
			foundSupportedInstrumenter = true
			break
		}
	}

	if !foundSupportedInstrumenter {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, ObservabilityInstrumenter, fmt.Errorf("instrumenter must be one of %v", supportedInstrumenters())))
	}

	foundSupportedInstrumenter = false
	for _, instrumenter := range s.Instrumenters {
		if instrumenter.Name() == s.ObservabilityInstrumenter {
			foundSupportedInstrumenter = true
			break
		}
	}
	if !foundSupportedInstrumenter {
		mErr = multierror.Append(mErr, fmt.Errorf("instrumenter %s not provided", s.ObservabilityInstrumenter))
	}
	return mErr
}

// PreRun implements run.PreRunner
func (s *Service) PreRun() error {
	for _, instrumenter := range s.Instrumenters {
		if instrumenter.Name() == s.ObservabilityInstrumenter {
			s.delegate = instrumenter
			break
		}
	}
	return s.delegate.PreRun()
}

// Serve implements run.GroupService
func (s *Service) Serve() error {
	return s.delegate.Serve()
}

// GracefulStop implements run.GroupService
func (s *Service) GracefulStop() {
	s.delegate.GracefulStop()
}

// Tracer implements observability.Tracerer
func (s *Service) Tracer() Tracer {
	return s.delegate.Tracer()
}

// SpanFromContext implements observability.Contexter
func (s *Service) SpanFromContext(ctx context.Context) Span {
	return s.delegate.SpanFromContext(ctx)
}

// Middleware implements observability.Middlewareer
func (s *Service) Middleware() func(http.Handler) http.Handler {
	return s.delegate.Middleware()
}

// Transport implements observability.Transporter
func (s *Service) Transport(transport http.RoundTripper) (http.RoundTripper, error) {
	return s.delegate.Transport(transport)
}
