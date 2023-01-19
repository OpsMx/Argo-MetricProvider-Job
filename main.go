package main

import (
	"context"
	"fmt"
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
	log.SetLevel(log.DebugLevel)
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
	basePath := "/etc/config/"
	resourceNames, err := checkPatchabilityReturnResources(c)
	if err != nil {
		return err
	}
	isDryRun, err := dryRunMode()
	if err != nil {
		return err
	}
	if isDryRun {
		log.Info("starting the dry-run")
	}

	log.Info("starting the runAnalysis function")
	errcode, errrun := runAnalysis(c, resourceNames, basePath, isDryRun)
	if errrun != nil {
		errMsg := errrun.Error()
		if isDryRun {
			errMsg = fmt.Sprintf("dryRunDetails\n Error: %s", errMsg)
		}
		err := patchJobError(context.TODO(), c.kubeclientset, resourceNames.jobName, errMsg)
		if err != nil {
			log.Error("an error occurred while patching the error from run analysis")
			return err
		}
		log.Infof("an error occurred while processing the request - %s", errMsg)
	}
	if errcode != 0 {
		logNon0CodeExit(errcode)
	}
	return nil

}

func main() {
	config, err := rest.InClusterConfig()
	checkError(err)

	clientset, err := kubernetes.NewForConfig(config)
	checkError(err)

	httpclient := NewHttpClient()

	clients := newClients(clientset, httpclient)

	err = runner(clients)
	checkError(err)
}
