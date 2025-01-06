# Local development with docker

**Objective:** Elevate Developer Experience (DX) for local development with `docker` by enabling `.test` FQDNs with TLS support and services auto-wire.

This solution, powered by a streamlined `docker compose` stack, enables developers to get rid of random ports binding and seamlessly replicate application infrastructures of virtually any complexity using containerized environments and single entrypoint locally.

**Status:** In active development _(breaking changes expected)_

> **NOTE:** project public name and internal reference might change.

**List of features:**
- OS-agnostic (Linux/macOS) developer experience
- seamless routing on **host** and in **containers** using same IP and FQDNs
- reserved [.test](https://en.wikipedia.org/wiki/.test) TLD support
- layer 4 and 7 **load-balancer** with **TLS termination** by `traefik`
- automated TLS certificate generation by `mkcert`
- private **DNS nameserver** by `dnsmasq`
- flexible UI to manage `docker` resources by `portainer`
- configuration customization support
- framework to auto-wire separate application stacks
- full offline support, except for the initial run
- various examples and more ...

```mermaid
graph LR
    subgraph docker[Docker]
        dNet(docker0<br>172.17.0.1)
        dDNS(DNS)
        cDNS[dnsmasq<br>172.20.0.2]
        cLB[traefik<br>172.20.0.3]
        cSvc@{shape: procs, label: "svc1, svc2, svcN<br>192.168.22.0/24"}
    end

    subgraph host[Host]
        hDNS(DNS)
        hResolver(resolver)
        hApp@{shape: procs, label: "browser, cli<br>192.168.0.123"}
    end

    hApp ---> |resolve svc1.test| hDNS ---> |use| hResolver ---> |resolve .test| dNet
    hApp --> |open svc1.test| dNet

    dDNS -.-> |inherit| hDNS

    dNet ---> |resolve| cDNS -.-> |172.17.0.1| dNet
    dNet ---> |route| cLB ---> |route| cSvc

    cSvc ---> |resolve svc2.test| dDNS
    cSvc ---> |open svc2.test| dNet

    linkStyle default stroke:#999
    classDef default fill:#eee,color:#999,stroke:#999

    classDef env fill:#f9f9f9,color:#333,stroke:#ccc,rx:5,ry:5,font-weight:bold
    classDef app fill:#f7f2e6,stroke:#db9a16
    classDef dns fill:#f5ebeb,stroke:#c66
    classDef container fill:#ebf5eb,stroke:#6c6
    classDef docker0 fill:#ebebf5,stroke:#66c

    class docker,host env
    class hApp app
    class dNet docker0
    class dDNS,hDNS,hResolver dns
    class cDNS,cLB,cSvc container
```

Such setup enhances security by limiting internal resources exposure while enabling seamless FQDN resolution from the
host and from the container, ensuring efficient and secure communication.

**Tested on:**

  - **Linux**: Debian (11/12), Ubuntu (24), Fedora (38/40)
  - **macOS**: Rancher Desktop (qemu/vz), colima (qemu/vz)


## Usage

**Prerequisites:**

- `docker` server is up and running
- `docker compose` plugin is installed
- `mkcert` binary is installed

Step-by-step configuration:

1. prepare root CA certificate:

    create and trust `mkcert`-managed CA by **host** trusted stores:

    > **NOTE:** It will ask several times for your password (in CLI and GUI), as it uses `sudo`.

    ```console
    $ mkcert -install
    ```

    allow to use **host** root CA:

    ```console
    $ cp "$(mkcert -CAROOT)"/rootCA*.pem certs/
    ```

    > **ALTERNATIVE:** It's possible to run stack without sharing root CA, as it would be auto-generated within `./certs` directory.
    >
    > After that, it must be added to host trusted store as:
    >
    > ```console
    > $ CAROOT=./certs mkcert -install
    > ```
    >
    > **NOTE:** Using this approach, root CA must be re-trusted after on  re-create, eg. after cleanup.

1. build and start `docker compose` stack:

    ```console
    $ docker compose build

    $ docker compose up --wait
    ```

1. tail `docker compose` logs for observability _(best in other terminal window/tab)_:

    ```console
    $ docker compose logs -f
    ```

1. setup **host** system: _(once per setup)_

    - configure [Linux](#setup-linux) host
    - configure [macOS](#setup-macos) host
    - run [setup validation](#setup-validation)
    - check [troubleshooting](#setup-troubleshooting) if needed

1. visit http://local.test to kickstart your developer experience

    Explore examples and guides to help you make the most of this project.

1. celebrate, share your experience and report findings


## Setup

By default, exposed ports of the `dnsmasq` and `traefik` containers are bound to the `172.17.0.1` IP address,
which corresponds to the `docker0` interface on Linux. Therefore code snippets below will use it by default.

Detect configured default docker IP:

```console
$ docker network inspect bridge --format='{{(index .IPAM.Config 0).Gateway}}'
```

> **NOTE:** If no match - use [configuration override](#setup-configuration-override) to match it and update code snippets before running.

**Host** uses custom DNS resolver to resolve `.test` FQDNs using `172.17.0.1` nameserver, that is inherited by **docker**.

### Setup: configuration override

Certain properties could be overridden using `.env` file.

Inspect [.env.dist](.env.dist) file for possible override properties.

1. make a copy of `.env.dist` file:

    ```console
    $ cp .env.dist .env
    ```

    > **NOTE:** file `.env` is git-ignored

1. uncomment and adjust envvars to your needs

1. recreate `docker compose` stack

    ```console
    $ docker compose down
    $ docker composer up --wait
    ```
---

### Setup: Linux

Running `docker` on **Linux** is native, and requires minimal **host** adjustments:

1. setup [DNS resolver](#setup-linux--dns-resolver)


#### Setup: Linux / DNS resolver

> **NOTE**: `systemd-resolved` is listening on `127.0.0.53:53` address by default, so no conflict expected.

Step-by-step configuration:

1. update `systemd-resolved` to forward `.test` requests directly to `172.17.0.1`:

    ```console
    $ sudo mkdir -p /etc/systemd/resolved.conf.d

    $ cat <<EOF | sudo tee /etc/systemd/resolved.conf.d/test.conf
    [Resolve]
    DNS=172.17.0.1
    Domains=~test
    EOF
    ```

1. enable and restart `systemd-resolved` service:

    ```console
    $ sudo systemctl enable systemd-resolved
    $ sudo systemctl restart systemd-resolved
    $ sudo systemctl status systemd-resolved
    ```

1. let `systemd-resolve` to manage `/etc/resolv.conf`:

    ```console
    $ sudo ln -snf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf
    ```

---

### Setup: macOS

Running `docker` on **macOS** relies on an intermediate **docker VM** and a bit advanced **host** adjustments.

**macOS** constraints for `docker`:
- there is no `docker0` network interface
- no route to `172.17.0.1` IP address by default
- once network is disconnected or switched off, non-localhost DNS traffic is denied
- use `vz` virtualization with `virtiofs` and `rosetta` options for **docker VM** running on **Apple Silicon**
- using network alias for routing, eg. `ifconfig lo0 alias 10.254.254.254`, is error prone and should not be used
- **docker VM** MUST be bi-directionally reachable via dedicated IP address, eg. `192.168.205.x`
- **docker VM** is managed by one of:

    > **NOTE:** run ONE **docker VM** at a time to avoid unexpected networking issues.

  - [Docker Desktop](https://www.docker.com/products/docker-desktop/)
    - not yet tested

  - [Rancher Desktop](https://docs.rancherdesktop.io)
    - **vz** [rootless, privileged]: OK
    - **qemu** [rootless]: FAIL - no routing, host DNS resolver is not reachable
    - **qemu** [privileged]: OK

    > **NOTE:** `privileged` consumes extra IP from the router, while `rootless` uses NAT

  - [colima](https://github.com/abiosoft/colima)
    - **qemu** [rootless, privileged]: OK, if started using `--network-address` flag
    - **vz** [rootless, privileged]: OK, if started using `--network-address` flag

  - [lima](https://github.com/lima-vm/lima)
    - not tested directly

Required **host** adjustments:

1. setup [DNS resolver and proxy](#setup-macos--dns-resolver-and-proxy)
1. setup one of routing methods:
    - [docker VM IP](#setup-macos--routing--docker-vm-ip)
    - [static routing](#setup-macos--routing--static)
    - [automated routing](#setup-macos--routing--automation)
1. bundle [Root CA certificate](#setup-macos--root-ca-certificate)

---

#### Setup: macOS / DNS resolver and proxy

> **NOTE**: due to **macOS** network behavior, it's highly recommended to use localhost DNS proxy.
If network is disconnected or disabled, **macOS** denies non-localhost DNS traffic.
Just imagine, you're on the plane w/o internet and `.test` domains stops to work.

Step-by-step configuration:

1. update system resolver to forward `.test` requests to `localhost`:

    ```console
    $ sudo mkdir -p /etc/resolver
    $ echo 'nameserver 127.0.0.1' | sudo tee /etc/resolver/test
    ```

1. install `dnsmasq` package to act as DNS proxy:

    ```console
    $ brew install dnsmasq
    ```

1. update `dnsmasq` config to forward requests to **upsteam** DNS in **docker**:

    ```console
    $ cat <<EOF | tee "$(brew --prefix)/etc/dnsmasq.d/proxy.conf"
    listen-address=127.0.0.1
    server=/.test/172.17.0.1
    EOF

    $ dnsmasq --test
    ```

1. restart DNS proxy server and reset cache:

    ```console
    $ sudo brew services restart dnsmasq
    $ sudo killall -HUP mDNSResponder
    ```

> **NOTE:** `.test` FQDNs won't be resolved unless routing setup is completed.

---

#### Setup: macOS / routing / docker VM IP

This method binds exposed ports of `traefik` and `dnsmasq` containers directly to **docker VM** IP address.

**Pros:**
- easy to start
- simplest approach
- good for static setup
- no routing to manage
- not affected by OS restart

**Cons:**
- no auto-detect for **docker VM** IP - must be explicitly detected
- requires `DOCKER_DEFAULT_IP` override using `.env` file
- won't survive **docker VM** recreation with IP change - must re-configure

Step-by-step configuration:

1. detect IP address of **docker VM**:

    - [Rancher-Desktop](#issue-detect-ip-address-of-rancher-desktop-vm)
    - [colima](#issue-detect-ip-address-of-colima-vm)

    Eg, assume it is `192.168.205.2`

1. add or update following line to `.env` file:

    ```ini
    DOCKER_DEFAULT_IP=192.168.205.2
    ```

1. recreate `docker compose` stack:

    ```console
    $ docker compose down
    $ docker composer up --wait
    ```

1. replace `.test` server IP with **docker VM** IP for DNS proxy and validate:

    ```console
    $ sed -i -e '/^server=/s,.test/.*,.test/192.168.205.2,' "$(brew --prefix)/etc/dnsmasq.d/proxy.conf"

    $ dnsmasq --test
    ```

1. restart DNS proxy server and reset cache:

    ```console
    $ sudo brew services restart dnsmasq
    $ sudo killall -HUP mDNSResponder
    ```

---

#### Setup: macOS / routing / static

This method adds static route to default docker IP using **docker VM** IP as gateway.

**Pros:**
- easy to start
- simple approach
- good for static setup
- no docker default IP override by default

**Cons:**
- **docker VM** IP considered ephemeral and must be explicitly detected
- static routing - 1 rule per 1 CIDR
- route will be lost on **host** restart - must define service to auto-start
- route will be lost on **docker VM** restart - must re-run service
- manual cleanup needed

Step-by-step configuration:

1. detect **docker VM** IP address for your setup:

    - [Rancher-Desktop](#issue-how-to-detect-ip-address-of-rancher-desktop-vm)
    - [colima](#issue-how-to-detect-ip-address-of-colima-vm)

    Eg, assume it is `192.168.205.2`

1. create `local.test-docker-route` service file:

    ```console
    $ cat <<EOF | sudo tee /Library/LaunchDaemons/local.test-docker-route.plist
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0">
      <dict>
          <key>Label</key>
          <string>local.test-docker-route</string>
          <key>RunAtLoad</key>
          <true/>
          <key>ProgramArguments</key>
          <array>
              <string>/sbin/route</string>
              <string>-n</string>
              <string>add</string>
              <string>-net</string>
              <string>172.17.0.1/32</string>
              <string>192.168.205.2</string>
          </array>
      </dict>
    </plist>
    EOF

    $ plutil -lint /Library/LaunchDaemons/local.test-docker-route.plist

    $ sudo chown root:wheel /Library/LaunchDaemons/local.test-docker-route.plist
    $ sudo chmod 644 /Library/LaunchDaemons/local.test-docker-route.plist
    ```

1. start `local.test-docker-route` service and verify routing table:

    ```console
    $ sudo launchctl load -w /Library/LaunchDaemons/local.test-docker-route.plist
    ```

1. verify routing table contains `172.17.0.1` record:

    ```console
    $ netstat -rnf inet
    ```

> **NOTE:** `launchctl unload` DOES NOT remove route, this has to be done manually
>
> ```console
> $ sudo launchctl unload /Library/LaunchDaemons/local.test-docker-route.plist
>
> $ sudo route delete 172.17.0.1
> ```

---

#### Setup: macOS / routing / automation

This method uses [chipmk/docker-mac-net-connect](https://github.com/chipmk/docker-mac-net-connect/) binary
to create a tunnel to **docker VM** and automatically manage routing rules to **docker** subnets.

For more details please refer to the original documentation.

**Pros:**
- provides auto-discovery for docker subnets
- automated routing management
- no need to detect **docker VM** IP
- `homebrew` provides default service to auto-start
- good for dynamic/experimental setup
- no overrides by default

**Cons:**
- additional dependency to manage
- uses `10.33.33.1` IP on **host** for tunnel interface, possible collisions on VPN
- uses privileged docker socket at `/var/run/docker.sock` by default
- rootless socket could be used either as symlink to privileged one, or by customizing `homebrew` service with `DOCKER_HOST` envvar

Step-by-step configuration:

1. install `docker-mac-net-connect` binary:

    ```console
    $ brew install chipmk/tap/docker-mac-net-connect
    ```

1. start `docker-mac-net-connect` service:

    ```console
    $ sudo brew services start docker-mac-net-connect
    ```

    > **NOTE:** it requires some seconds to pick up and configure **host** and **docker VM**

1. check logs for issues:

    ```console
    $ sudo tail -n 100 "$(brew --prefix)/var/log/docker-mac-net-connect"/*.log
    ```

    <details>
      <summary>successful logs</summary>

    ```log
    ...
    {"status":"Status: Downloaded newer image for ghcr.io/chipmk/docker-mac-net-connect/setup:v0.1.3"}
    Creating WireGuard interface chip0
    Assigning IP to WireGuard interface
    Configuring WireGuard device
    DEBUG: (utun0) peer(Q/Ir…4tD4) - Received handshake initiation
    DEBUG: (utun0) peer(Q/Ir…4tD4) - Sending handshake response
    Adding iptables NAT rule for host WireGuard IP
    Setup container complete
    Adding route for 172.17.0.0/16 -> utun0 (bridge)
    Adding route for 172.18.0.0/16 -> utun0 (bridge)
    DEBUG: (utun0) Watching Docker events
    ...
    ```

    </details>

    <details>
      <summary>failure logs</summary>

    ```log
    ...
    DEBUG: (utun0) Setting up Wireguard on Docker Desktop VM
    Image doesn't exist locally. Pulling...
    ERROR: (utun0) Failed to setup VM: failed to pull setup image: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
    ...
    ```

    Could be fixed using one of the following methods:
    - configure **docker VM** to create privileged docker socket _(see Rancher-Desktop issue)_
    - symlink user docker socket to the privileged socket path.

        ```console
        sudo ln -nfs ~/.rd/docker.sock /var/run/docker.sock

          - or -

        sudo ln -nfs ~/.colima/<NAME>/docker.sock /var/run/docker.sock
        ```

    - customize plist service to utilize `DOCKER_HOST` envvar with correct endpoint from `docker context ls` command.

    </details>

    > **NOTE:** after successful setup, **docker VM** will have new `chip0` interface.

1. verify routing table contains `172.17` record:

    ```console
    $ netstat -rnf inet
    ```

> **NOTE:** `lima`-based VM might require extra `iptables` rule - see [chipmk/docker-mac-net-connect#26](https://github.com/chipmk/docker-mac-net-connect/issues/26#issuecomment-1565238036) for details.

---


#### Setup: macOS / root CA certificate

Non-native **macOS** CLI binaries rely on `ca-certificates` package and do not use system `Keychain` to verify certificates,
therefore new **root CA** certificate must be propagated manually.

Resync `ca-certificates` package, it will pull trusted CAs with recent updates from Mozilla:

```console
$ HOMEBREW_NO_AUTO_UPDATE=1 \
  HOMEBREW_NO_INSTALL_CLEANUP=1 \
  HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK=1 \
  brew reinstall ca-certificates
```

Verify that it's properly bundled:

```console
$ SUBJECT=$(openssl x509 -in "$(mkcert -CAROOT)/RootCA.pem" -noout -subject)

$ awk -v decoder='openssl x509 -noout -subject 2>/dev/null' '/BEGIN/{close(decoder)};{print | decoder}' \
  < "$(brew --prefix)/etc/ca-certificates/cert.pem" \
  | grep -Fx "${SUBJECT}"
```

Add environment variables to `.profile` or `.zprofile` file:

```console
export CURL_CA_BUNDLE="$(brew --prefix)/etc/ca-certificates/cert.pem"
export SSL_CERT_FILE="${CURL_CA_BUNDLE}"
```

> **NOTE:** each language/framework might use different envvars. Check [examples/](examples/) to discover details and adjust list accordingly.

---

### Setup: validation

After host system is configured, `.test` FQDNs must be resolved and accessible from host and from container.

1. verify `.test` FQDN is correctly resolved:

    from **Linux** host:

    ```console
    $ nslookup local.test
    $ host -t A local.test
    $ dig A local.test +short
    ```

    from **macOS** host: _(must specify localhost as NS for non-native apps)_

    ```console
    $ dscacheutil -q host -a name local.test
    $ nslookup local.test 127.0.0.1
    $ host -t A local.test 127.0.0.1
    $ dig @127.0.0.1 A local.test +short
    ```

    from container:

    ```console
    $ docker run --rm --network proxy busybox nslookup local.test
    ```

    > **NOTE:** using `colima` VM - container test might fail for the first time - please retry.

1. verify `.test` could be accessed:

    from host:
    ```console
    $ curl -LisS http://whoami.local.test
    ```

    from container:
    ```console
    $ docker run --rm --network proxy \
      -v proxy_certs:/certs:ro -e CURL_CA_BUNDLE=/certs/ca-bundle.crt alpine \
      sh -ec 'apk add -q curl; curl -LisS http://whoami.local.test'
    ```

    <details>
      <summary>expected 'curl' response</summary>

    ```
    HTTP/1.1 301 Moved Permanently
    Location: https://whoami.local.test/
    Date: Fri, 03 Jan 2025 15:07:13 GMT
    Content-Length: 17

    HTTP/2 200
    content-type: text/plain; charset=utf-8
    date: Fri, 03 Jan 2025 15:07:13 GMT
    content-length: 371

    Hostname: cf28b8dc9cd3
    IP: 127.0.0.1
    IP: ::1
    IP: 172.18.0.7
    RemoteAddr: 172.18.0.3:36358
    GET / HTTP/1.1
    Host: whoami.local.test
    User-Agent: curl/8.11.1
    Accept: */*
    Accept-Encoding: gzip
    X-Forwarded-For: 192.168.205.1
    X-Forwarded-Host: whoami.local.test
    X-Forwarded-Port: 443
    X-Forwarded-Proto: https
    X-Forwarded-Server: c6f62641a182
    X-Real-Ip: 192.168.205.1
    ```
    <details>

---

### Setup: troubleshooting

#### Issue: how to detect IP address of `Rancher-Desktop` VM?

IP address could be discovered from `rd1` or `vznat` network interfaces using `rdctl` CLI tool.

Usually it's `192.168.205.2`.

```console
$ rdctl shell ls -1 /sys/class/net \
  | grep -E '^(rd1|vznat)$' \
  | xargs -I{} -r -- rdctl shell ip addr show dev {} \
  | awk '/inet / {print $2}' \
  | cut -d/ -f1
```

#### Issue: how to detect IP address of `colima` VM?

IP address could be discovered using `colima` CLI tool or from `col0` network interface.

Usually it's one of `192.168.205.x`.

for `default` instance:

```console
$ colima list -j | jq -r .address
```

for `MY_NAME` instance:

```console
$ colima list -j -p MY_NAME | jq -r .address
```

#### Issue: could not resolve `.test` host - ERR_NAME_NOT_RESOLVED

There might be few reasons to fail, check one by one:

1. check if `docker compose` stack is up and running

    If not - start or recreate the stack.

1. check if configured `DOCKER_DEFAULT_IP` is reachable:

    ```console
    $ _TARGET_IP=$( source .env 2>/dev/null || :; echo "${DOCKER_DEFAULT_IP:-172.17.0.1}")
    $ echo "${_TARGET_IP}"

    $ ping -c1 "${_TARGET_IP}"
    ```

    If not - check if `_TARGET_IP` is correct, routing is configured, firewall is not blocking, **docker VM** has dedicated IP.

1. check if `.test` FQDN is resolved using configured `DOCKER_DEFAULT_IP`

    ```console
    $ nslookup a.test "${_TARGET_IP}"
    ```

    If not - check if `dnsmasq` container is up and running.

1. check if `local.test` FQDN is resolved using DNS resolver

    ```console
    # Linux
    $ nslookup a.test

    # macOS
    $ nslookup a.test 127.0.0.1
    ```

    If not - check if DNS resolver configured properly.


#### Issue: your connection is not private - ERR_CERT_COMMON_NAME_INVALID

There is no **Subject Alternative Name** (SAN) on certificate, that matches current FQDN.

Specify `TLS_SANS_EXTRA=my.custom.test` value using `.env` file to fix this issue.

> **NOTES:**
> - X.509 wildcard only go one level deep - `*.*.local.test` won't match `a.b.local.test`
> - X.509 wildcard on TLD level is not allowed - `*.test` won't be recognized


#### Issue: [macOS] could not open `.test` URL - ERR_ADDRESS_UNREACHABLE

**macOS** puts extra _(imo useless)_ security measures for non-native apps to restrict access to local CIDR blocks.

Fix it by enabling **local network access** for each browser/application you're using to communicate with docker to workaround it.

> System Settings > Privacy & Security > Local Network

More on [support.google.com](https://support.google.com/chrome/thread/299849666/err-address-unreachable-only-for-local-network-resources) thread.
