package main

import (
	"context"
	"net/http"

	argo "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/typed/rollouts/v1alpha1"
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

func newClients(rolloutClient argo.ArgoprojV1alpha1Client, kubeclientset kubernetes.Interface, client http.Client) *Clients {
	return &Clients{
		RolloutClient: rolloutClient,
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
	log.Info("starting the runAnalysis function")
	errcode, errrun := runAnalysis(c, resourceNames, basePath)
	if errrun != nil {
		errMsg := errrun.Error()
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

	// Create an Argo Rollouts clientset
	rolloutsClientset, err := argo.NewForConfig(config)
	checkError(err)

	clients := newClients(*rolloutsClientset, clientset, httpclient)

	log.Info("starting the runner function")
	err = runner(clients)
	checkError(err)
}
