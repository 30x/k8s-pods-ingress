# Overview

This project contains a proof of concept ingress controller for Kubernetes.  There are a few things that are done
differently for this ingress than your typical
[Kubernetes Ingress controller](http://kubernetes.io/v1.1/docs/user-guide/ingress.html):

* This version does pod-level routing instead of service-level routing
* This version does not use the
[Kubernetes Ingress Resource](http://kubernetes.io/v1.1/docs/user-guide/ingress.html#the-ingress-resource) definitions
and instead uses pod-level annotations to wire things up *(This design was not my doing and was dictated by an internal
design)*

The current state of this project is that this is a proof of concept driven by an internal design.  That design and
this implementation could change at any time.

# Design

This ingress controller is written in Go and upon startup, this controller will get a list of all pods across all
namespaces having the `microservice` label set to `true`.  These pods are then analyzed for the necessary wiring
configuration annotations:

* `trafficHosts`: This is a space delimited array of hosts that the pod should serve traffic for *(required)*
* `publicPaths`: This is the space delimited array of public paths that the pod should serve traffic for *(required, the
value's format is `{PORT}:{PATH}` where `{PORT}` corresponds to the container port serving the traffic for the `{PATH)`*

Once we've found all pods that are properly configured as microservices, we generate an nginx configuration file.

This initial list of pods is then cached and from this point forward we listen for pod events and alter our internal
cache accordingly based on the pod event.  *(The idea here was to allow for an initial hit to pull all pods but to then
to use the events for as quick a turnaround as possible.)*  Events are processed in 2 second chunks.

Each pod can expose one or more services using multiple entries in the `publicPaths` annotation.  All paths/services are
exposed for each of the hosts listed in the `trafficHosts` annotation.  _(So if you have a trafficHosts of `host1 host2`
and a `publicPaths` of `80:/ 3000:/nodejs`, you would have 4 separate nginx location blocks: `host1/ -> {PodIP}:80`,
`host2/ -> {PodIP}:80`, `host1/nodejs -> {PodIP}:3000` and `host2/nodejs -> {PodIP}:3000`  Right now there is no way to
associate specific paths to specific hosts but it may be something we support in the future.)_

# Example

Let's assume you've already deployed the ingress controller.  *(If you haven't, feel free to look at the
[Building and Running](#building-and-running) section of the documentation.)*  When the ingress starts up, nginx is
configured and started on your behalf.  The generated `/etc/nginx/nginx.conf` that the ingress starts with looks like
this *(assuming you do not have any deployed microservices)*:

``` nginx
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {
  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
daemon on;
```

This configuration will tell nginx to listen on port `80` and all requests for unknown hosts will be closed, as if there
was no server listening for traffic.  *(This approach is better than reporting a `404` because a `404` says someone is
there but the request was for a missing resource while closing the connection says that the request was for a server
that didn't exist, or in our case a request was made to a host that our ingress is unaware of.)*

Now that we know how the ingress spins up nginx initially, let's deploy a _microservice_ to Kubernetes.  To do that, we
will be packaging up a simple Node.js application that prints out the environment details of its running container,
including the IP address(es) of its host.  To do this, we will build a Docker image, publish the Docker image and then
create a Kubernetes ReplicationController that will deploy one pod representing our microservice.

**Note:** All commands you see in this demo assume you are already within the `demo` directory.
These commands are written assuming you are running docker at `192.168.64.1:5000` so please adjust your Docker commands
accordingly.

First things first, let's build our Docker image using `docker build -t nodejs-k8s-env .`, tag the Docker image using
`docker tag -f nodejs-k8s-env 192.168.64.1:5000/nodejs-k8s-env` and finally push the Docker image to your Docker
registry using `docker push 192.168.64.1:5000/nodejs-k8s-env`.  At this point, we have built and published a Docker
image for our microservice.

The next step is to deploy our microservice to Kubernetes but before we do this, let's look at the ReplicationController
configuration file to see what is going on.  Here is the `rc.yaml` we'll be using to deploy our microservice
*(Same as `demo/rc.yaml`)*:

``` yaml
apiVersion: v1
kind: ReplicationController
metadata:
  name: nodejs-k8s-env
  labels:
    name: nodejs-k8s-env
spec:
  replicas: 1
  selector:
    name: nodejs-k8s-env
  template:
    metadata:
      labels:
        name: nodejs-k8s-env
        # This marks the pod as a microservice
        microservice: "true"
      annotations:
        # This says that only traffic for the "test.k8s.local" host will be routed to this pod
        trafficHosts: "test.k8s.local"
        # This says that only traffic for the "/nodejs" path and its sub paths will be routed to this pod, on port 3000
        publicPaths: "3000:/nodejs"
    spec:
      containers:
      - name: nodejs-k8s-env
        image: whitlockjc/nodejs-k8s-env
        env:
          - name: PORT
            value: "3000"
        ports:
          - containerPort: 3000
```

When we deploy our microservice usin `kubectl create -f rc.yaml`, the ingress will notice that we now have one pod
running that is marked as a microservice.  If you were tailing the logs, or you were to review the content of
`/etc/nginx/nginx.conf` in the container, you should see that it now reflects that we have a new microservice
deployed:

``` nginx
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  server {
    listen 80;
    server_name test.k8s.local;

    location /nodejs {
      proxy_set_header Host $host;
      # Pod nodejs-k8s-env-eq7mh
      proxy_pass http://10.244.69.6:3000;
    }
  }

  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
```

This means that if someone requests `http://test.k8s.local/nodejs`, assuming you've got `test.k8s.local` pointed to the
edge of your Kubernetes cluster, it should get routed to the proper pod *(`nodejs-k8s-env-eq7mh` in our example)*.  If
everything worked out properly, you should see output like this:

``` json
{
  "env": {
    "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    "HOSTNAME": "nodejs-k8s-env-eq7mh",
    "PORT": "3000",
    "KUBERNETES_PORT": "tcp://10.100.0.1:443",
    "KUBERNETES_PORT_443_TCP": "tcp://10.100.0.1:443",
    "KUBERNETES_PORT_443_TCP_PROTO": "tcp",
    "KUBERNETES_PORT_443_TCP_PORT": "443",
    "KUBERNETES_PORT_443_TCP_ADDR": "10.100.0.1",
    "KUBERNETES_SERVICE_HOST": "10.100.0.1",
    "KUBERNETES_SERVICE_PORT": "443",
    "VERSION": "v5.8.0",
    "NPM_VERSION": "3",
    "HOME": "/root"
  },
  "ips": {
    "lo": "127.0.0.1",
    "eth0": "10.244.69.6"
  }
}
```

Now that's cool and all but what happens when we scale our application?  Let's scale our microservice to `3` instances
using `kubectl scale --replicas=3 replicationcontrollers nodejs-k8s-env`.  Your `/etc/nginx/nginx.conf` should look
something like this:

``` nginx
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  # Upstream for /nodejs traffic on test.k8s.local
  upstream microservice1866206336 {
    # Pod nodejs-k8s-env-eq7mh
    server 10.244.69.6:3000;
    # Pod nodejs-k8s-env-yr1my
    server 10.244.69.8:3000;
    # Pod nodejs-k8s-env-oq9xn
    server 10.244.69.9:3000;
  }

  server {
    listen 80;
    server_name test.k8s.local;

    location /nodejs {
      proxy_set_header Host $host;
      # Upstream microservice1866206336
      proxy_pass http://microservice1866206336;
    }
  }

  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
```

The big change between the one pod microservice and the N pod microservice is that now the nginx configuration uses
the nginx [upstream](http://nginx.org/en/docs/http/ngx_http_upstream_module.html) to do load balancing across the N
different pods.  And due to the default load balancer in nginx being round-robin based, requests for
`http://test.k8s.local/nodejs` should return a different payload for each request showing that you are indeed
hitting each individual pod.

I hope this example gave you a better idea of how this all works.  If not, let us know how to make it better.

# Building and Running

If you're testing this outside of Kubernetes, you can just use `go build` followed by
`KUBE_HOST=... ./k8s-pods-ingress`.  If you're building this to run on Kubernetes, you'll need to do the following:

* `CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o k8s-pods-ingress .`
* `docker build ...`
* `docker tag ...`
* `docker push ...`

*(The `...` are there because your Docker comands will likely be different than mine or someone else's)*  We have an
example `rc.yaml` for deploying the k8s-pods-ingress to Kubernetes.  Here is how I test locally:

* `CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o k8s-pods-ingress .`
* `docker build -t k8s-pods-ingress .`
* `docker tag -f k8s-pods-ingress 192.168.64.1:5000/k8s-pods-ingress`
* `docker push 192.168.64.1:5000/k8s-pods-ingress`
* `kubectl create -f rc.yaml`

**Note:** This ingress is written to be ran within Kubernetes but for testing purposes, it can be ran outside of
Kubernetes.  When ran outside of Kubernetes, you will have to set the `KUBE_HOST` environment variable to point to the
Kubernetes API.  *(Example: `http://192.168.64.2:8080`)*  When ran outside the container, nginx itself will not be
started and its configuration file will not be written to disk, only printed to stdout.  This might change in the future
but for now, this support is only as a convenience.

# Credit

This project was largely based after the `nginx-alpha` example in the
[kubernetes/contrib](https://github.com/kubernetes/contrib/tree/master/ingress/controllers/nginx-alpha) repository.
