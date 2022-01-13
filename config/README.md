## topology-tester

This is a small readme on the topology-tester application.

The container can be downloaded from docker: `basvanbeek/demoapp`

The bundled files hold some config for a K8s deployment, use and adjust at
need.

I've used envsubst to template the service config. You can see in `deploy.sh`
I create 6 different deployments / services based on this single image. 

In this config set-up we have the following services in the demo namespace: 
- alpha
- beta
- gamma
- delta
- epsilon
- zeta

The following service endpoints are available:

```go
router.Methods("GET").Path("/headers/{percentage}").HandlerFunc(ep.setDoubleHeaders)
router.Methods("GET").Path("/errors/{percentage}").HandlerFunc(ep.setErrors)
router.Methods("GET").Path("/latency/{duration}").HandlerFunc(ep.setLatency)
router.Methods("GET").Path("/crash/{message}").HandlerFunc(ep.crash)
router.Methods("GET").Path("/local/{concurrency}/latency/{duration}").HandlerFunc(ep.emulateConcurrency)
router.Methods("GET").PathPrefix("/proxy/{service}").HandlerFunc(ep.proxy)
router.Methods("GET").PathPrefix("/").HandlerFunc(ep.echoHandler)
```

| variable | type | examples |
| --- | --- | --- |
| percentage | integer | 50 (means 50%)
| duration   | duration or integer | 60ms or 60, 1s or 1000, 1m20s
| message    | string | oopsie
| concurrency | enum(serial,mixed,parallel) | mixed
| service | host[:port] | svcb, svcd:8000

So each service has these... by using the `/proxy/{service}` path segment you
can have services hop requests between each other.

Typically, you create a K8s ingress gateway and point it to the alpha service.

This allows you to call the service from outside like this:

Example:

http://demo.example.org/proxy/beta/proxy/delta/proxy/alpha/proxy/zeta/

This call will eventually land at `zeta` which will run the echo handler.

But the path it takes will be:

```
client -> ingress gw -> alpha -> beta -> delta -> alpha -> zeta
```

An example response is:
```json
{
  "service": "zeta",
  "statusCode": 200,
  "traceID": "6a09777e4a449cd4e2b58b00ead6615f",
  "headers": {
    "Accept": [
      "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"
    ],
    "Accept-Encoding": [
      "gzip, deflate"
    ],
    "Accept-Language": [
      "en-US,en;q=0.9"
    ],
    "Cache-Control": [
      "max-age=0"
    ],
    "Content-Length": [
      "0"
    ],
    "Proxied-By": [
      "alpha",
      "beta",
      "delta",
      "alpha"
    ],
    "Sec-Gpc": [
      "1"
    ],
    "Upgrade-Insecure-Requests": [
      "1"
    ],
    "User-Agent": [
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.63 Safari/537.36"
    ],
    "X-B3-Parentspanid": [
      "240740c849cd1875"
    ],
    "X-B3-Sampled": [
      "0"
    ],
    "X-B3-Spanid": [
      "941810e19f4f9bf7"
    ],
    "X-B3-Traceid": [
      "6a09777e4a449cd4e2b58b00ead6615f"
    ],
    "X-Envoy-Attempt-Count": [
      "1"
    ],
    "X-Forwarded-For": [
      "10.142.0.4, 127.0.0.1, 127.0.0.1, 127.0.0.1, 127.0.0.1"
    ],
    "X-Forwarded-Proto": [
      "http"
    ],
    "X-Request-Id": [
      "e6cb1201-c0c8-416c-ab1e-ed3cba877ab4"
    ]
  }
```

You will notice the `Proxied-By` header listing all services that reverse
proxied the request to finally reach the service as listed in the `service`
property.

Next to being able to set default settings at boot time for:
- echo handler & proxy handler latency
- echo handler & proxy handler error percentage
- echo handler double header percentage

You can also adjust these for a service at runtime (and yes you can proxy these)

Example of adjusting the error percentage for `zeta`:

http://demo.example.org/proxy/zeta/errors/50

Example response:
```json
{
  "service": "zeta",
  "statusCode": 200,
  "traceID": "07611a44444839cbc718bc7f0c8919d0",
  "message": "errors percentage set to: 50%"
}
```

# Istio

By default, Istio doesn't sample all requests. If using Istio 1.11 or up you
can utilize the telemetry APIs to specifically change the sample rate for these
services. See: https://istio.io/latest/docs/tasks/observability/telemetry/

If having a specific Istio cluster for these apps, you can adjust the global
sample rate and config Zipkin following the information found here:

- https://istio.io/latest/docs/tasks/observability/distributed-tracing/zipkin/
- https://istio.io/latest/docs/tasks/observability/distributed-tracing/mesh-and-proxy-config/#customizing-trace-sampling

If you don't want to do this you can also use `curl` to do the requests and
force sampling:

```sh
curl -H 'X-B3-Sampled: 1' http://demoapp.tetrate.com/
```

Another option is to install an extension in your browser and send the header
through that. Example extension: https://modheader.com/
