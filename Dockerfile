# Copyright (c) Bas van Beek 2022.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:alpine as builder
ENV GOPATH /go

COPY . $GOPATH/src/github.com/basvanbeek/topology-tester
WORKDIR $GOPATH/src/github.com/basvanbeek/topology-tester

RUN CGO_ENABLED=0 go build -o /build/topology-tester cmd/server/main.go

FROM scratch

COPY --from=builder /build/topology-tester /topology-tester

ENTRYPOINT ["/topology-tester"]
