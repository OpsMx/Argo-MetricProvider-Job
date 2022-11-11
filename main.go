package main

import (
	"context"
	"net/http"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.InfoLevel)
}

func newClients(kubeclientset kubernetes.Interface, client http.Client) *Clients {
	return &Clients{
		kubeclientset: kubeclientset,
		client:        client,
	}
}

func NewHttpClient() http.Client {
	c := http.Client{
		Timeout: httpConnectionTimeout,
	}
	return c
}

func runner(c *Clients) error {
	resourceNames, err := checkPatchabilityReturnResources(c)
	if err != nil {
		return err
	}
	if errrun := runAnalysis(c, resourceNames); errrun != nil {
		err := patchJobError(c.kubeclientset, context.TODO(), resourceNames.jobName, errrun.Error())
		if err != nil {
			log.Error("An error occurred while patching the error from runAnalysis")
			return err
		}
	}
	return nil

}

func main() {
	config, err := rest.InClusterConfig()
	logErrorExit1(err)

	clientset, err := kubernetes.NewForConfig(config)
	logErrorExit1(err)

	httpclient := NewHttpClient()

	clients := newClients(clientset, httpclient)

	log.Info("Starting the runner function")
	err = runner(clients)
	logErrorExit1(err)
}
