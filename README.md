# kallax

## Installation
```shell script
$: docker service create --name=kallax \
      --mode global \
      --constraint node.labels.role.kallax==yes \
      --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
      --publish published=5353,target=5353,protocol=udp,mode=host \
      faryon93/kallax:latest
```