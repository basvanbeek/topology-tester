#!/bin/bash

# Copyright (c) Bas van Beek 2022.
# Copyright (c) Tetrate, Inc 2021.
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

set -a

# set defaults if not available
: ${IMAGE:=basvanbeek/demoapp:latest}
: ${NS:=obs-tester}
: ${INSTRUMENTER:=zipkin}
: ${DEPLOYMENT:=deployment.yaml}
: ${ZIPKIN_ENDPOINT:=http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans}
: ${ZIPKIN_SAMPLE_RATE:=1.0}
: ${SKYWALKING_ENDPOINT:=oap.default.svc.cluster.local:11800}
: ${SKYWALKING_SAMPLE_RATE:=1.0}

# bootstrap 6 services
if [[ $INSTRUMENTER = "zipkin" ]]
then
  DEPLOYMENT=deployment.yaml
else
  DEPLOYMENT=deployment-skywalking.yaml
fi

SVCNAME=alpha   envsubst < $DEPLOYMENT | kubectl apply -f -
SVCNAME=beta    envsubst < $DEPLOYMENT | kubectl apply -f -
SVCNAME=gamma   envsubst < $DEPLOYMENT | kubectl apply -f -
SVCNAME=delta   envsubst < $DEPLOYMENT | kubectl apply -f -
SVCNAME=epsilon envsubst < $DEPLOYMENT | kubectl apply -f -
SVCNAME=zeta    envsubst < $DEPLOYMENT | kubectl apply -f -

set +a
