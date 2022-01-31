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

package service

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/basvanbeek/topology-tester/pkg/observability"
	"github.com/gorilla/mux"
	"github.com/tetratelabs/multierror"
	"github.com/tetratelabs/run"

	"github.com/basvanbeek/topology-tester/pkg"
)

const (
	flagDuration       = "ep-duration"
	flagErrors         = "ep-errors"
	flagHeaders        = "ep-headers"
	flagHandleFailures = "ep-handle-failures"

	errProxyService   pkg.Error = "invalid or no proxy service set"
	errPercentage     pkg.Error = "expected percentage value between 0 and 100"
	errDuration       pkg.Error = "expected a zero or positive duration"
	errConcurrency    pkg.Error = "invalid or no concurrency type set"
	errInternal       pkg.Error = "internal service failure occurred"
	errHandleFailures pkg.Error = "expected boolean value for handling failures"
)

// Endpoints implements a run.Config compatible group of Endpoints which will
// register themselves on the provided http service, using the provided Zipkin
// tracer to instrument themselves.
type Endpoints struct {
	// dependencies
	Instrumenter observability.Instrumenter

	ServiceName string

	handler http.Handler
	tracer  observability.Tracer

	// service globals protected by mutex mtx
	mtx            sync.RWMutex
	errors         int32
	headers        int32
	duration       time.Duration
	handleFailures bool
}

// Name implements run.Unit.
func (ep *Endpoints) Name() string {
	return "endpoints"
}

// FlagSet implements run.Config.
func (ep *Endpoints) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("Endpoint options")

	flags.Int32Var(&ep.errors, flagErrors, ep.errors,
		`Percentage of errors on echo handler`)

	flags.Int32Var(&ep.headers, flagHeaders, ep.headers,
		`Percentage of double headers on echo handler`)

	flags.DurationVar(&ep.duration, flagDuration, ep.duration,
		`Duration of a request on echo handler`)

	flags.BoolVar(&ep.handleFailures, flagHandleFailures, ep.handleFailures,
		`Handle failures when proxying and return OK to requestor`)

	return flags
}

// Validate implements run.Config.
func (ep *Endpoints) Validate() error {
	var mErr error

	if ep.errors < 0 || ep.errors > 100 {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, flagErrors, errPercentage),
		)
	}
	if ep.headers < 0 || ep.headers > 100 {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, flagHeaders, errPercentage),
		)
	}
	if ep.duration < 0 {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, flagDuration, errDuration),
		)
	}

	return mErr
}

// PreRun implements run.PreRunner.
func (ep *Endpoints) PreRun() error {
	if ep.Instrumenter == nil || ep.Instrumenter.Tracer() == nil {
		return errors.New("missing tracer to attach to")
	}

	// create our service router
	router := mux.NewRouter()
	router.Methods("GET").Path("/headers/{percentage}").HandlerFunc(ep.setDoubleHeaders)
	router.Methods("GET").Path("/errors/{percentage}").HandlerFunc(ep.setErrors)
	router.Methods("GET").Path("/graceful/{handleFailures}").HandlerFunc(ep.setHandleFailures)
	router.Methods("GET").Path("/latency/{duration}").HandlerFunc(ep.setLatency)
	router.Methods("GET").Path("/crash/{message}").HandlerFunc(ep.crash)
	router.Methods("GET").Path("/local/{concurrency}/latency/{duration}").HandlerFunc(ep.emulateConcurrency)
	router.Methods("GET").PathPrefix("/proxy/{service}").HandlerFunc(ep.proxy)
	router.Methods("GET").PathPrefix("/").HandlerFunc(ep.echoHandler)
	ep.tracer = ep.Instrumenter.Tracer()

	ep.handler = ep.Instrumenter.Middleware()(router)

	return nil
}

// Handler returns an HTTP handler that can be attached to an HTTP service.
// The handler holds a router to the endpoints with the sub handlers.
func (ep *Endpoints) Handler() http.Handler {
	return ep.handler
}

var (
	_ run.Config    = (*Endpoints)(nil)
	_ run.PreRunner = (*Endpoints)(nil)
)
