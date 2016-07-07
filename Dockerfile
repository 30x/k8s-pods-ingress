FROM nginx:stable-alpine

MAINTAINER Jeremy Whitlock <jwhitlock@apache.org>

LABEL Description="A general purpose router for Kubernetes."

# Prepare the environment
RUN apk update \
  # Install the CA Certificates for SSL usage
  && apk add --no-cache ca-certificates \
  # Update the CA Certificates
  && update-ca-certificates \
  # Remove the nginx log symlinks to give more control over how logging is controlled
  && unlink /var/log/nginx/access.log \
  && unlink /var/log/nginx/error.log

COPY k8s-router /

CMD ["/k8s-router"]
