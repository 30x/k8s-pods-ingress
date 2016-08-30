# k8s-router Releases

## 1.0.4 (2016-08-29)

* Fixed bug where we assumed that a running `Pod` meant the `Pod` had an IP _(Issue #38)_

## 1.0.3 (2016-08-29)

* Updated the initial startup to always succeed even if the `Pod` state would generate an invalid `/etc/nginx/nginx.conf` _(Issue #37)_

