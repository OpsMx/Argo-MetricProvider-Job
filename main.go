package main

import (
	"context"
	"errors"
	"os"

	argoclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
)

// TODO - Check the feasibilty/correctness of passing the metric name in the ResourceNames struct
// TODO- Remove prints and add log lines everywhere
func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	//TODO - Set from an env var maybe?
	log.SetLevel(log.InfoLevel)
}

func newClients(kubeclientset kubernetes.Interface, argoclient argoclientset.Interface) *Clients {
	return &Clients{
		kubeclientset: kubeclientset,
		argoclientset: argoclient,
	}
}

func checkPatchabilityReturnResources(c *Clients) (ResourceNames, error) {

	podName, ok := os.LookupEnv("MY_POD_NAME")
	if !ok {
		return *new(ResourceNames), errors.New("environment variable my_pod_name not set")
	}

	jobName, err := getJobNameFromPod(c, podName)
	if err != nil {
		return *new(ResourceNames), err
	}

	_, err = c.kubeclientset.BatchV1().Jobs(defaults.Namespace()).Patch(context.TODO(), jobName, types.StrategicMergePatchType, []byte(`{}`), metav1.PatchOptions{}, "status")
	if err != nil {
		log.Errorf("%s", err)
		log.Errorln("Cannot patch to Job, check the service account has the right permissions and the pod Name is correct")
		return *new(ResourceNames), err
	}
	resourceNames := ResourceNames{
		podName: podName,
		jobName: jobName,
	}
	return resourceNames, nil
}

func runner(c *Clients) error {
	//TODO - Use errors
	resourceNames, err := checkPatchabilityReturnResources(c)
	if err != nil {
		return err
	}

	if errrun := runAnalysis(c, resourceNames); errrun != nil {
		err := patchJobError(c, context.TODO(), resourceNames.jobName, errrun.Error())
		if err != nil {
			log.Error("--")
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

	argoclient, err := argoclientset.NewForConfig(config)
	logErrorExit1(err)

	clients := newClients(clientset, argoclient)

	err = runner(clients)
	logErrorExit1(err)
}
