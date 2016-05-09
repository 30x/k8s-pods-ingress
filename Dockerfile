FROM gcr.io/google_containers/nginx

MAINTAINER Jeremy Whitlock <jwhitlock@apache.org>

LABEL Description="A Pod-based ingress/router for Kubernetes."

COPY k8s-pods-ingress /

CMD ["/k8s-pods-ingress"]
