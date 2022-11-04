package main

import (
	"context"
	"os"

	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func logErrorExit1(err error){
	if err != nil {
		log.Infof("Inside the exit 1 block")
		log.Error(err)
		os.Exit(1)
	}
}


func getAnalysisRunNameFromPod(p *Clients, ctx context.Context ,podName string) (string,error){
	//TODO - Introduce more checks, remove prints
	ns:= defaults.Namespace()
	jobName := getJobNameFromPod(podName)
	log.Infof("The job name is %s", jobName)

	job, err := p.kubeclientset.BatchV1().Jobs(ns).Get(ctx,jobName,metav1.GetOptions{})
	if err != nil {
		return "",err
	}
	parent :=  job.OwnerReferences[0]
	var analysisRunName string
	if parent.Kind == "AnalysisRun"{
		analysisRunName = parent.Name
	}
	return analysisRunName,nil

}


func getJobNameFromPod(podName string) string {
// TODO- Retrieve data from the last hyphen and use error if required
	return podName[:len(podName)-6]

}