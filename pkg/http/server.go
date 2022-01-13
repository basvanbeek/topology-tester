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

package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/tetratelabs/multierror"
	"github.com/tetratelabs/run"

	"github.com/basvanbeek/topology-tester/pkg"
)

const (
	flagListenAddress = "http-listen-address"

	defaultListenAddress = ":8000"
)

var (
	_ run.Config  = (*Service)(nil)
	_ run.Service = (*Service)(nil)
)

// Service implements a run.Group compatible HTTP Server.
type Service struct {
	ListenAddress string

	*http.Server
	l net.Listener
}

// Name implements run.Unit.
func (s *Service) Name() string {
	return "http"
}

// FlagSet implements run.Config.
func (s *Service) FlagSet() *run.FlagSet {
	if s.ListenAddress == "" {
		s.ListenAddress = defaultListenAddress
	}
	if s.Server == nil {
		s.Server = &http.Server{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
	}
	flags := run.NewFlagSet("HTTP server options")

	flags.StringVarP(
		&s.ListenAddress,
		flagListenAddress, "a",
		s.ListenAddress,
		`HTTP server listen address, e.g. ":443" or "localhost:80"`)

	return flags
}

// Validate implements run.Config.
func (s *Service) Validate() error {
	var mErr error

	if s.ListenAddress != "" {
		if _, _, err := net.SplitHostPort(s.ListenAddress); err != nil {
			mErr = multierror.Append(mErr,
				fmt.Errorf(pkg.FlagErr, flagListenAddress, err))
		}
	} else {
		mErr = multierror.Append(mErr,
			fmt.Errorf(pkg.FlagErr, flagListenAddress, pkg.ErrRequired))
	}

	return mErr
}

// Serve implements run.Service.
func (s *Service) Serve() (err error) {
	s.l, err = net.Listen("tcp", s.ListenAddress)
	if err != nil {
		return err
	}
	return s.Server.Serve(s.l)
}

// GracefulStop implements run.Service.
func (s *Service) GracefulStop() {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	if s.Server != nil {
		_ = s.Server.Shutdown(ctx)
	}
	if s.l != nil {
		_ = s.l.Close()
	}
}
