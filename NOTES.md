instead of alias to `lo0` add route to lima vm:

```
DOCKER0_IP=$(docker network inspect bridge --format "{{range .IPAM.Config}}{{.Gateway}}{{end}}")
```

```
sudo route -n add -net "${DOCKER0_IP}" 192.168.205.2
```
