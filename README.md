# kallax
Kallax is an DNS-SD based service-discovery server for Docker Swarm. 

## Installation
```shell script
$: docker service create --name=kallax \
      --mode global \
      --constraint node.labels.role.kallax==yes \
      --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
      --publish published=5353,target=5353,protocol=udp,mode=host \
      faryon93/kallax:latest
```

## Configure Service
```shell script
$: docker service update --label-add="{\"node_exporter\": {\"port\": 9100, \"net\":\"<prom-net-id>\"}}"
```

## Split DNS
```shell script
$: cat /etc/dnsmasq.conf
# forward service discovery queries to kallax
server=/kallax.local/<ip-of-kallax-server>#5353

# disable caching: always ask dns server
cache-size=0

# only accessibly by docker containers (ip of docker_gwbridge)
listen-address=172.18.0.1
bind-interfaces

$: docker service update --dns-add 172.18.0.1
```
