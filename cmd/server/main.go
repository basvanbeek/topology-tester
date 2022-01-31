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

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/basvanbeek/topology-tester/internal/service"
	pkghttp "github.com/basvanbeek/topology-tester/pkg/http"
	pkgobs "github.com/basvanbeek/topology-tester/pkg/observability"
	pkgskywalking "github.com/basvanbeek/topology-tester/pkg/observability/skywalking"
	pkgzipkin "github.com/basvanbeek/topology-tester/pkg/observability/zipkin"

	"github.com/tetratelabs/run"
	"github.com/tetratelabs/run/pkg/signal"
)

const (
	defaultServiceName       = "demosvc"
	defaultHTTPListenAddress = ":8000"

	defaultZipkinAddress        = "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans"
	defaultSkywalkingOAPAddress = "oap.default.svc.cluster.local:11800"
	defaultSampleRate           = 1.0
	defaultSingleHostSpans      = true
)

func main() {
	// we take the serviceName from an environment variable as we need
	// this information to be available prior to run.Group bootstrap.
	serviceName := os.Getenv("SVCNAME")
	if serviceName == "" {
		serviceName = defaultServiceName
	}
	serviceInstanceName := os.Getenv("HOSTNAME")
	if serviceInstanceName == "" {
		serviceInstanceName = serviceName
	}

	g := run.Group{
		Name:     serviceName,
		HelpText: "Flexible HTTP service to create observed topologies",
	}

	// init with sensible defaults
	svcObs := &pkgobs.Service{
		ObservabilityInstrumenter: "zipkin",
		Instrumenters: []pkgobs.InstrumenterService{&pkgzipkin.Service{
			Servicename:     serviceName,
			Address:         defaultZipkinAddress,
			SampleRate:      defaultSampleRate,
			SingleHostSpans: defaultSingleHostSpans,
		},
			&pkgskywalking.Service{
				Servicename:         serviceName,
				ServiceInstanceName: serviceInstanceName,
				Address:             defaultSkywalkingOAPAddress,
				SampleRate:          defaultSampleRate,
			},
		},
	}

	svcEndpoints := &service.Endpoints{
		ServiceName:  serviceName,
		Instrumenter: svcObs,
	}
	svcHTTP := &pkghttp.Service{
		ListenAddress: defaultHTTPListenAddress,
	}
	g.Register(
		new(signal.Handler),
		svcObs,
		svcEndpoints,
		svcHTTP,
		run.NewPreRunner(serviceName, func() error {
			svcHTTP.Handler = svcEndpoints.Handler()
			return nil
		}),
	)

	if err := g.Run(); err != nil {
		fmt.Printf("%s exit: %v\n", g.Name, err)
		if !errors.Is(err, run.ErrRequestedShutdown) {
			// We had an actual fatal error.
			os.Exit(-1)
		}
	}
}
