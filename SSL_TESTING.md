# SSL Testing


## How to configure?

### Curl

`curl` relies on standard CA certificated bundling, so it's enough to specify these envvars:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`
    - `CURL_CA_BUNDLE=/certs/ca-bundle.pem`

### Wget

`wget` does not rely on standard CA certificated bundling, therefore `~/.wgetrc` config could be adjusted:

```shell
ca_certificate = /certs/ca-bundle.pem
```

### Java

`java` uses it's own trust store, so certificate has to be explicitly imported using alias:

```shell
$ keytool -cacerts -storepass changeit -list -alias proxy_root_ca >/dev/null 2>&1 \
    || keytool -cacerts -storepass changeit -importcert -noprompt -file /certs/proxy-ca.crt -alias proxy_root_ca
```

### NodeJS

`node` relies on standard CA certificated bundling, so it's enough to specify these envvars:
    - `NODE_EXTRA_CA_CERTS=/certs/ca-bundle.pem`

### GoLang

`go` relies on standard CA certificated bundling, so it's enough to specify these envvars:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`

### Python

`python` relies on standard CA certificated bundling, so it's enough to specify these envvars:
    - `SSL_CERT_FILE=/certs/ca-bundle.pem`
    - `CURL_CA_BUNDLE=/certs/ca-bundle.pem`
    - `HTTPLIB2_CA_CERTS=/certs/ca-bundle.pem`

#### Python: examples

1. package `requests` uses `REQUESTS_CA_BUNDLE` or `CURL_CA_BUNDLE` envvars:

    ```shell
    $ python3 -c 'import requests;res=requests.get("https://whoami.local.test");res.raise_for_status();print(res.content.decode())'
    ```

1. package `httpx` uses `SSL_CERT_FILE` or `SSL_CERT_DIR` ennvars:

    ```shell
    $ python3 -c 'import httpx;res=httpx.get("https://whoami.local.test");res.raise_for_status();print(res.text)'
    ```

1. package `httplib2` uses `HTTPLIB2_CA_CERTS` ennvar:

    ```shell
    $ python3 -c 'import httplib2;res=httplib2.Http().request("https://whoami.local.test");print(res[1].decode())'
    ```

1. package `urllib` uses `SSL_CERT_FILE` or `SSL_CERT_DIR` envvars:

    ```shell
    $ python3 -c 'import urllib.request; print(urllib.request.urlopen("https://whoami.local.test").read().decode())'
    ```

1. package `urllib3` uses `SSL_CERT_FILE` or `SSL_CERT_DIR` envvars:

    ```shell
    $ python3 -c 'import urllib3; print(urllib3.request("GET", "https://whoami.local.test").data.decode())'
    ```

1. package `http.client` uses `SSL_CERT_FILE` or `SSL_CERT_DIR` envvars:

    ```shell
    $ python3 -c 'import http.client; con=http.client.HTTPSConnection("whoami.local.test"); con.request("GET", "/"); res=con.getresponse(); print(res.read().decode())'
    ```
