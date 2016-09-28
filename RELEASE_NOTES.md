# k8s-router Releases

## TBD

* Enhanced `Pod` routability validation to check the routing path container against the list of container ports _(Issue #12)_

## 1.0.5 (2016-09-02)

* Fixed bug where error check didn't happen in the right spot

## 1.0.4 (2016-08-29)

* Fixed bug where we assumed that a running `Pod` meant the `Pod` had an IP _(Issue #38)_

## 1.0.3 (2016-08-29)

* Updated the initial startup to always succeed even if the `Pod` state would generate an invalid `/etc/nginx/nginx.conf` _(Issue #37)_

