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
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/basvanbeek/topology-tester/pkg"
)

type response struct {
	Service string      `json:"service"`
	Code    int         `json:"statusCode"`
	TraceID string      `json:"traceID"`
	Message string      `json:"message,omitempty"`
	Error   pkg.Error   `json:"error,omitempty"`
	Headers http.Header `json:"headers,omitempty"`
}

func (ep *Endpoints) writeResponse(ctx context.Context, w http.ResponseWriter, res response) {
	res.Service = ep.ServiceName
	res.TraceID = ep.traceID(ctx)
	w.Header().Add("Content-Type", "application/json")
	if res.Code > 0 {
		w.WriteHeader(res.Code)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		log.Printf("error while writing http response: %v", err)
	}
}

func (ep *Endpoints) traceID(ctx context.Context) string {
	return ep.Instrumenter.SpanFromContext(ctx).TraceID()
}
