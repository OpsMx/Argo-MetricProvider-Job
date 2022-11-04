package main

import (
	argoclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)


const (
	AnalysisPhasePending      = "Pending"
	AnalysisPhaseRunning      = "Running"
	AnalysisPhaseSuccessful   = "Successful"
	AnalysisPhaseFailed       = "Failed"
	AnalysisPhaseError        = "Error"
	AnalysisPhaseInconclusive = "Inconclusive"
)


type Clients struct {
	kubeclientset kubernetes.Interface
	argoclientset argoclientset.Interface
}

//TODO- Change to export maybe?
type ResourceNames struct{
	podName string
	jobName string
	analysisRunName string	

}

//TODO- Change to export maybe?
type CanaryDetails struct {
	jobName string
	metricName string
	canaryId string
	gateUrl string
	reportUrl string
	phase string
	value string
}

//Check the feasibilty of merging this struct with the CanaryDetails one
//TODO- Change to export maybe?
type FinalStatus struct {
	jobName string
	metricName string
	phase string
	message string
}
