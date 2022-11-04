package main

import (
	"context"
	"os"

	argoclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
)

//TODO - Check the feasibilty/correctness of passing the metric name in the ResourceNames struct
//TODO- Remove prints and add log lines everywhere

func init(){
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	//TODO - Set from an env var maybe?
	log.SetLevel(log.InfoLevel)
}

func newClients(kubeclientset kubernetes.Interface, argoclient argoclientset.Interface ) *Clients {
	return &Clients{
		kubeclientset: kubeclientset,
		argoclientset: argoclient,
	}
}

func runner(c *Clients) error{
	ns:= defaults.Namespace()
	ctx:=context.TODO()
	//TODO - Use errors
	podName := os.Getenv("MY_POD_NAME")
	jobName:= getJobNameFromPod(podName)
	analysisRunName,err := getAnalysisRunNameFromPod(c, ctx, podName)
	if err!= nil {
		return err
	}

	resourceNames := ResourceNames{
		podName: podName,
		jobName: jobName,
		analysisRunName: analysisRunName,
	}

	if errrun := runAnalysis(c,resourceNames); errrun != nil {
		ar,err := c.argoclientset.ArgoprojV1alpha1().AnalysisRuns(ns).Get(ctx,analysisRunName,metav1.GetOptions{})
		if err != nil{
			return err
		}

		fs := FinalStatus{
			metricName : ar.Spec.Metrics[0].Name,
			phase: "Running",
			message : errrun.Error(),
			jobName: jobName,
		}

		err= patchError(c,context.TODO(),analysisRunName,fs)
		if err!= nil {
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

	clients := newClients(clientset,argoclient)

	err = runner(clients)
	log.Infof("Error in runner")
	logErrorExit1(err)
}