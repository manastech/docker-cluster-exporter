package main

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	labels       = []string{"name", "stack", "service"}
	memUsageDesc = prometheus.NewDesc(
		"docker_container_memory_usage_bytes",
		"Total memory usage in bytes",
		labels, nil,
	)

	memReservationDesc = prometheus.NewDesc(
		"docker_container_memory_reservation_bytes",
		"Memory reserved for the container in bytes",
		labels, nil,
	)

	memLimitDesc = prometheus.NewDesc(
		"docker_container_memory_limit_bytes",
		"Memory limit for the container in bytes",
		labels, nil,
	)
	docker *client.Client
)

type dockerCollector struct {
}

func (c dockerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- memUsageDesc
	ch <- memReservationDesc
	ch <- memLimitDesc
}

func (c dockerCollector) Collect(ch chan<- prometheus.Metric) {
	cgroupVersion := detectCgroupVersion()
	containers, err := docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for _, container := range containers {
		name := containerName(&container)
		stack, service := stackService(&container)

		usageBytes := containerUsageBytes(container.ID, cgroupVersion)
		totalCache := containerTotalCacheBytes(container.ID, cgroupVersion)

		ch <- prometheus.MustNewConstMetric(
			memUsageDesc,
			prometheus.GaugeValue,

			float64(usageBytes-totalCache),
			name, stack, service,
		)

		inspect, err := docker.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Fatal(err)
		}

		ch <- prometheus.MustNewConstMetric(
			memReservationDesc,
			prometheus.GaugeValue,

			float64(inspect.HostConfig.MemoryReservation),
			name, stack, service,
		)

		ch <- prometheus.MustNewConstMetric(
			memLimitDesc,
			prometheus.GaugeValue,

			float64(inspect.HostConfig.Memory),
			name, stack, service,
		)
	}
}

func detectCgroupVersion() string {
	return "v1" // FIXME: implement
}

func containerTotalCacheBytes(containerId string, cgroupVersion string) int64 {
	if cgroupVersion == "v2" {
		log.Fatal("cgroups v2 not yet supported") // FIXME: implement
	}
	memoryStat, err := readMapFile("/host/sys/fs/cgroup/memory/docker/" + containerId + "/memory.stat")
	if err != nil {
		log.Fatal(err)
	}
	return memoryStat["total_cache"]
}

func containerUsageBytes(containerId string, cgroupVersion string) int64 {
	if cgroupVersion == "v2" {
		log.Fatal("cgroups v2 not yet supported") // FIXME: implement
	}
	usageBytes, err := readSimpleValueFile("/host/sys/fs/cgroup/memory/docker/" + containerId + "/memory.usage_in_bytes")
	if err != nil {
		log.Fatal(err)
	}
	return usageBytes
}

func readSimpleValueFile(path string) (int64, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return -1, err
	}
	str := strings.TrimSpace(string(data))

	value, err := strconv.ParseInt(str, 10, 64)
	return value, err
}

func readMapFile(path string) (map[string]int64, error) {
	values := make(map[string]int64)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')

		switch err {
		case io.EOF:
			return values, nil

		case nil:
			parts := strings.Split(strings.TrimSpace(line), " ")
			value, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil, err
			}
			values[parts[0]] = value

		default:
			return nil, err
		}

		if err == io.EOF {
			return values, nil
		}

	}
}

func containerName(container *types.Container) string {
	name := container.Names[0]
	if name[0] == '/' {
		return name[1:]
	}

	return name
}

func stackService(container *types.Container) (string, string) {
	rancherStackService := container.Labels["io.rancher.stack_service.name"]
	if rancherStackService != "" {
		stackAndService := strings.Split(rancherStackService, "/")
		return stackAndService[0], stackAndService[1]
	}

	swarmStackService := container.Labels["com.docker.swarm.service.name"]
	if swarmStackService != "" {
		swarmNamespace := container.Labels["com.docker.stack.namespace"]
		if swarmNamespace != "" {
			service := strings.TrimPrefix(swarmStackService, swarmNamespace+"_")
			return swarmNamespace, service
		}
		// Fallback to no namespace available - we're almost making stuff up here
		stackAndService := strings.SplitN(swarmStackService, "_", 2)
		if len(stackAndService) > 1 {
			return stackAndService[0], stackAndService[1]
		}
		return "-", swarmStackService
	}

	stack := container.Labels["com.docker.compose.project"]
	service := container.Labels["com.docker.compose.service"]
	return stack, service
}

func firstLabel(container *types.Container, labels ...string) string {
	for _, label := range labels {
		value := container.Labels[label]
		if value != "" {
			return value
		}
	}

	return ""
}

func main() {
	var err error
	docker, err = client.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}

	reg := prometheus.NewRegistry()
	reg.Register(dockerCollector{})

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Println("Listening... :9476")
	log.Fatal(http.ListenAndServe(":9476", nil))
}
