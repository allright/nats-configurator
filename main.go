package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func readJsonFile(name string) (map[string]interface{}, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error: '%v' open file: %v", err, name))
	}
	v := make(map[string]any)
	err = json.Unmarshal(data, &v)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error: '%v' parse json file: %v", err, name))
	}
	return v, nil
}

func writeJsonFile(name string, v map[string]interface{}) error {
	out, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return errors.New(fmt.Sprintf("error: '%v' encode file: %v", err, name))
	}

	err = os.WriteFile(name, out, os.ModePerm)
	if err != nil {
		return errors.New(fmt.Sprintf("error: '%v' write file: %v", err, name))
	}
	return nil
}

func getNatsPid(name string) (int, error) {
	pidData, err := os.ReadFile(name)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("error: '%v' read pid file: '%v' ", err, name))
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		return 0, errors.New(fmt.Sprintf("error: '%v' parse pid file: '%v' ", err, name))
	}
	return pid, nil
}

var Version = "0.0.0"
var Hash = "xxxxxxxx"
var BuildDate = ""

func buildInfo() string {
	return fmt.Sprintf("version: %v, build date: %v, hash: %v", Version, BuildDate, Hash)
}

func main() {
	log.Printf("started nats-configurator %v", buildInfo())
	defer log.Printf("stopped nats-configurator %v", buildInfo())

	podIp := flag.String("pod-ip", "", "pod ip address")
	podName := flag.String("pod-name", "", "pod name")
	nameSpace := flag.String("namespace", "default", "namespace")
	podLabel := flag.String("pod-label", "", "each pod with this have this label, will be included in mesh")
	natsRoutesPort := flag.Int("nats-routes-port", 6222, "port for internal cluster data exchange between nats nodes")
	natsConfigTemplate := flag.String("nats-config-template", "", "nats-config file template")
	natsConfigFileName := flag.String("nats-config", "", "nats-config")
	natsPidFile := flag.String("nats-pid-file", "", "nats pid file, for signal about reload config")
	refreshInterval := flag.Duration("refresh-interval", 3*time.Second, "pod list refresh interval")

	flag.Parse()

	natsConfig, err := readJsonFile(*natsConfigTemplate)
	if err != nil {
		log.Fatal(err)
	}
	natsConfig["server_name"] = *podName

	err = writeJsonFile(*natsConfigFileName, natsConfig)
	if err != nil {
		log.Fatal(err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	routesPrev := make([]string, 0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	ticker := time.NewTicker(*refreshInterval)
	for {
		select {
		case <-interrupt:
			return

		case <-ticker.C:
			pid, err := getNatsPid(*natsPidFile)
			if err != nil {
				log.Print(err)
				break
			}

			//	kubectl get pods -n nats -lapp=nats-configurator -o wide
			pods, err := clientset.
				CoreV1().
				Pods(*nameSpace).
				List(context.TODO(), metav1.ListOptions{LabelSelector: *podLabel})
			if err != nil {
				log.Printf("error listing pods: %v", err)
				break
			}

			routes := make([]string, 0)
			for _, pod := range pods.Items {
				if len(pod.Status.PodIP) == 0 || pod.Status.PodIP == *podIp {
					continue
				}
				routes = append(routes, fmt.Sprintf("nats://%v:%v", pod.Status.PodIP, *natsRoutesPort))
			}

			cluster := natsConfig["cluster"]
			if val, ok := cluster.(map[string]any); ok {
				if reflect.DeepEqual(routesPrev, routes) {
					time.Sleep(*refreshInterval)
					continue
				} else {
					val["routes"] = routes
					routesPrev = routes
					log.Printf("updating routes: %v", routes)
				}
			} else {
				log.Fatalf("error modify routes: '%v'", err)
			}

			err = writeJsonFile(*natsConfigFileName, natsConfig)
			if err != nil {
				log.Fatal(err)
			}

			err = syscall.Kill(pid, syscall.SIGHUP)
			if err != nil {
				log.Fatalf("error send SIGHUP to nats-server: %v with pid: %v", err, pid)
			}
		}
	}
}
