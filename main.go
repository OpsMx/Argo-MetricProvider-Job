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
	analysispath := "/etc/config/provider/providerConfig"
	userPath := "/etc/config/secrets/user"
	gateUrlPath := "/etc/config/secrets/gate-url"
	sourceNamePath := "/etc/config/secrets/source-name"
	cdIntegrationPath := "/etc/config/secrets/cd-integration"
	templatePath := "/etc/config/templates/%s"
	resourceNames, err := checkPatchabilityReturnResources(c)
	if err != nil {
		return err
	}
	errcode, errrun := runAnalysis(c, resourceNames, analysispath, userPath, gateUrlPath, sourceNamePath, cdIntegrationPath, templatePath);
	if errrun != nil {
		err := patchJobError(c.kubeclientset, context.TODO(), resourceNames.jobName, errrun.Error())
		if err != nil {
			log.Error("an error occurred while patching the error from run analysis")
			return err
		}
		// logNon0CodeExit(1)
	}
	if errcode !=0{
		logNon0CodeExit(errcode)
	}
	return nil

}

func main() {
	config, err := rest.InClusterConfig()
	logErrorExit(err)

	clientset, err := kubernetes.NewForConfig(config)
	logErrorExit(err)

	httpclient := NewHttpClient()

	clients := newClients(clientset, httpclient)

	log.Info("Starting the runner function")
	err = runner(clients)
	logErrorExit(err)
}
