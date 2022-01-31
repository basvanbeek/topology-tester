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
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// setErrors allows one to set the percentage of error responses this service
// will generate on the main echoHandler.
func (ep *Endpoints) setErrors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	strErrors, ok := mux.Vars(r)["percentage"]
	if !ok {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errPercentage,
		})
		return
	}

	i, err := strconv.Atoi(strErrors)
	if err != nil {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errPercentage,
		})
		return
	}
	if i < 0 || i > 100 {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errPercentage,
		})
		return
	}
	ep.mtx.Lock()
	ep.errors = int32(i)
	ep.mtx.Unlock()

	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Message: fmt.Sprintf("errors percentage set to: %d%%", i),
	})
}

// setDoubleHeaders allows one to set the percentage of double headers this
// service will generate on the main echoHandler.
func (ep *Endpoints) setDoubleHeaders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	strErrors, ok := mux.Vars(r)["percentage"]
	if !ok {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errPercentage,
		})
		return
	}

	i, err := strconv.Atoi(strErrors)
	if err != nil {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errPercentage,
		})
		return
	}
	if i < 0 || i > 100 {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errPercentage,
		})
		return
	}
	ep.mtx.Lock()
	ep.headers = int32(i)
	ep.mtx.Unlock()

	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Message: fmt.Sprintf("double headers percentage set to: %d%%", i),
	})
}

// setLatency allows one to set the latency in miliseconds this service will
// generate on the main echoHandler.
func (ep *Endpoints) setLatency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	strErrors, ok := mux.Vars(r)["duration"]
	if !ok {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errDuration,
		})
		return
	}

	d, err := time.ParseDuration(strErrors)
	if err != nil {
		// not a duration string, let's see if it is a raw number...
		var i int
		if i, err = strconv.Atoi(strErrors); err != nil {
			// not a raw number either...
			ep.writeResponse(ctx, w, response{
				Code:  http.StatusBadRequest,
				Error: errDuration,
			})
			return
		}
		d = time.Duration(i) * time.Millisecond
	}

	if d < 0 {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errDuration,
		})
		return
	}

	ep.mtx.Lock()
	ep.duration = d
	ep.mtx.Unlock()

	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Message: fmt.Sprintf("duration set to: %s", d.String()),
	})
}

// setHandleFailures allows one to set behavior of this service's proxy handler.
// If set to true, a downstream error will not cascade into a failure by this
// event. Instead, it will mimick a service that is resilient to downstream
// issues and can report back successfully. If set to false, nothing is changed
// and the proxy handler will happily forward the reply from downstream.
func (ep *Endpoints) setHandleFailures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	handleFailures, ok := mux.Vars(r)["handleFailures"]
	if !ok {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errHandleFailures,
		})
		return
	}

	var h bool
	switch strings.ToLower(handleFailures) {
	case "1", "on", "yes", "y", "true", "t":
		h = true
	case "0", "off", "no", "n", "false", "f":
		h = false
	default:
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errHandleFailures,
		})
		return
	}

	ep.mtx.Lock()
	ep.handleFailures = h
	ep.mtx.Unlock()

	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Message: fmt.Sprintf("handle failures set to: %t", h),
	})
}

// crash instructs this service to crash with the provided method after 5
// seconds of receiving this directive.
func (ep *Endpoints) crash(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	msg := mux.Vars(r)["message"]
	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Message: "crashing in 5 seconds",
	})

	go func() {
		time.Sleep(5 * time.Second)
		panic("crash requested: " + msg)
	}()
}

// emulateConcurrency instructs this service to run 8 fake heavy local methods.
// The methods will take the provided duration as their run time. The
// concurrency argument will instruct these methods to run serial, in parallel,
// or mixed serial and parallel. The methods are instrumented as local spans,
// so they will show up in your trace graph.
func (ep *Endpoints) emulateConcurrency(w http.ResponseWriter, r *http.Request) {
	var (
		ctx  = r.Context()
		vars = mux.Vars(r)
		d    time.Duration
		err  error
	)
	if strErrors, ok := vars["duration"]; ok {
		d, err = time.ParseDuration(strErrors)
		if err != nil {
			// not a duration string, let's see if it is a raw number...
			var i int
			if i, err = strconv.Atoi(strErrors); err != nil {
				// not a raw number either...
				ep.writeResponse(ctx, w, response{
					Code:  http.StatusBadRequest,
					Error: errDuration,
				})
				return
			}
			d = time.Duration(i) * time.Millisecond
		}
		if d < 0 {
			ep.writeResponse(ctx, w, response{
				Code:  http.StatusBadRequest,
				Error: errDuration,
			})
			return
		}
	}

	// we will be emulating 8 heavy internal functions
	var wg sync.WaitGroup
	wg.Add(8)

	proc := func(i int) {
		defer wg.Done()
		span := ep.tracer.StartSpanFromContext(ctx, fmt.Sprintf("proc-%d", i))
		defer span.Finish()

		span.Tag("duration", d.String())
		time.Sleep(d)
	}

	c := vars["concurrency"]
	switch strings.ToLower(c) {
	case "serial":
		for i := 0; i < 8; i++ {
			proc(i)
		}
	case "mixed":
		for i := 0; i < 8; i++ {
			if i%2 == 0 {
				go proc(i)
				continue
			}
			proc(i)
		}
	case "parallel":
		for i := 0; i < 8; i++ {
			go proc(i)
		}
	default:
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errConcurrency,
		})
		return
	}

	// wait until all goroutines are finished
	wg.Wait()

	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Message: "ran several local spans",
	})
}

// proxy parses and strips the first /proxy/service:port directive from the path
// and reverse proxies the remaining path request to the targeted service.
// This allows us to hop from service to service by providing path chunks
// referencing the services.
//
// Example path: /proxy/svcf/proxy/svcd/proxy/svcb/errors/50
// This path will hop from app ingress to svdf, svcd, svcb, where this final
// svcb will receive an /errors/50 request to handle.
func (ep *Endpoints) proxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	host, ok := mux.Vars(r)["service"]
	if !ok {
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusBadRequest,
			Error: errProxyService,
		})
		return
	}

	ep.mtx.RLock()
	d := ep.duration
	e := ep.errors
	h := ep.handleFailures
	ep.mtx.RUnlock()

	// inject configured latency
	time.Sleep(d)

	if rand.Int31n(100) < e {
		// return error response...
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusInternalServerError,
			Error: errInternal,
		})
		return
	}

	r.Header = r.Header.Clone()
	r.Host = host // this is needed or Envoy will get confused where to route it
	r.Header.Add("Proxied-By", ep.ServiceName)
	var (
		svc  = fmt.Sprintf("http://%s", host)
		path = strings.TrimPrefix(r.URL.Path, "/proxy/"+host)
		u, _ = url.Parse(svc)
		p    = httputil.NewSingleHostReverseProxy(u)
	)

	p.Transport, _ = ep.Instrumenter.Transport(p.Transport)
	r.URL, _ = url.Parse(svc + path)

	if h {
		p.ModifyResponse = func(res *http.Response) error {
			if res.StatusCode == 200 {
				// proceed unaltered
				return nil
			}
			// let's mimick a service that did a client request which failed,
			// but due to nice business logic it is still able to handle
			// the failure gracefully and return success status itself.
			res.StatusCode = 200
			raw, _ := ioutil.ReadAll(res.Body)
			_ = res.Body.Close()
			ep.writeResponse(ctx, w, response{
				Code: http.StatusOK,
				Message: fmt.Sprintf(
					"%s called %s and got error return: %s",
					ep.ServiceName, svc+path, string(raw)),
			})
			// bail proxy logic, we returned details upstream ourselves
			return errors.New("bail")
		}
	}
	p.ServeHTTP(w, r)
}

// echoHandler returns the received request handlers, potentially setting double
// headers (for testing Envoy sidecars), or fail with an error. The method will
// take at least as long as the set latency. Double headers and errors will
// occur with the set percentages in the service.
func (ep *Endpoints) echoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// retrieve our behavioral config
	ep.mtx.RLock()
	d := ep.duration
	h := ep.headers
	e := ep.errors
	ep.mtx.RUnlock()

	// inject configured latency
	time.Sleep(d)

	if rand.Int31n(100) < e {
		// return error response...
		ep.writeResponse(ctx, w, response{
			Code:  http.StatusInternalServerError,
			Error: errInternal,
		})
		return
	}

	if rand.Int31n(100) < h {
		// set some double headers
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Content-Type", "application/json")
	}

	// emulate successful response, sending request headers received
	ep.writeResponse(ctx, w, response{
		Code:    http.StatusOK,
		Headers: r.Header,
	})
}
