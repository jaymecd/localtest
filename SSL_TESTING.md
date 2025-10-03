# SSL Testing


## How to configure?

### Curl

`curl` relies on standard CA certificated bundling, so it's enough to specify one of these envvars:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`
    - `CURL_CA_BUNDLE=/certs/ca-bundle.pem`

### Wget

`wget` does not rely on standard CA certificated bundling, therefore `~/.wgetrc` config could be adjusted:

```shell
ca_certificate = /certs/ca-bundle.pem
```

### Java

`java` uses it's own trust store, so root certificate has to be explicitly imported using alias:

```shell
$ keytool -cacerts -storepass changeit -importcert -noprompt -file /certs/ca-localtest.pem -alias ca-localtest
```

### NodeJS

`node` relies on standard CA certificated bundling, however it uses dedicated envvar:
    - `NODE_EXTRA_CA_CERTS=/certs/ca-bundle.pem`

### Go

`go` relies on standard CA certificated bundling, so it's enough to specify envvar:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`

### Python

`python` relies on standard CA certificated bundling, while different packages might prefer own envvar.

It's recommended to export these envvars:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`
    - `CURL_CA_BUNDLE=/certs/ca-bundle.pem`
    - `HTTPLIB2_CA_CERTS=/certs/ca-bundle.pem`

### Ruby

`ruby` relies on standard CA certificated bundling, so it's enough to specify envvar:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`
