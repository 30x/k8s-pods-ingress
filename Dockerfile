FROM gcr.io/google_containers/nginx

COPY k8s-pods-ingress /
COPY default-nginx.conf /etc/nginx/nginx.conf

CMD ["/k8s-pods-ingress"]
