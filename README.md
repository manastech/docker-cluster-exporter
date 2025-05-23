# docker-cluster-exporter

Export your Docker cluster's metrics to Prometheus.

## Usage

When running Docker with cgroups v2, you should expose the proper cgroup slice
to the container in the `/host` directory. You have to also set to `v2` the
`DOCKER_CLUSTER_CGROUP_VERSION` environment variable - the exporter doesn't
auto-detect it yet, and will default to cgroups v1.

This could be a `docker-compose.yml` file:

```
services:
  docker-cluster-exporter:
    image: manastech/docker-cluster-exporter:v2
    privileged: true
    environment:
      DOCKER_CLUSTER_CGROUP_VERSION: v2
    network_mode: host
    volumes:
    - /var/run/docker.sock:/var/run/docker.sock
    - /sys/fs/cgroup/docker.slice/docker-workload.slice/:/host/
    pid: host
```

In this case, Docker was instructed to place its containers into a systemd
slice. Your mileage may vary - the exporter will look for a
`docker-CONTAINER_ID.slice` directory in the `/host/` one.
