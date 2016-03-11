FROM gcr.io/google_containers/nginx

COPY k8s-pods-ingress /

CMD ["/k8s-pods-ingress"]
