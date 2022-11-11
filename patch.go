package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func patchJobCanaryDetails(kubeclient kubernetes.Interface, ctx context.Context, cd CanaryDetails) error {

	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("Canary ID: %s\nReport URL: %s\nGate URL: %s", cd.canaryId, cd.reportUrl, cd.gateUrl),
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(kubeclient, jobStatus, ctx, cd.jobName)
	if err != nil {
		return err
	}

	log.Infof("Successfully patched to Jobfor canary ID %s", cd.canaryId)
	return nil
}

func patchJobSuccessful(kubeclient kubernetes.Interface, ctx context.Context, cd CanaryDetails) error {

	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("Canary ID: %s\nReport URL: %s\nGate URL: %s\nScore: %s", cd.canaryId, cd.reportUrl, cd.gateUrl, cd.value),
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(kubeclient, jobStatus, ctx, cd.jobName)
	if err != nil {
		return err
	}
	log.Infof("Successfully patched to Job with the score for canary ID %s", cd.canaryId)
	return nil
}

func patchJobFailedInconclusive(kubeclient kubernetes.Interface, ctx context.Context, reason string, cd CanaryDetails, errCode int) error {
	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("Canary ID: %s\nReport URL: %s\nGate URL: %s\nScore: %s", cd.canaryId, cd.reportUrl, cd.gateUrl, cd.value),
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(kubeclient, jobStatus, ctx, cd.jobName)
	if err != nil {
		return err
	}
	err = patchForcefulFail(kubeclient, ctx, cd.jobName, reason)
	if err != nil {
		return err
	}
	log.Infof("Successfully patched to Job with the score for canary ID %s", cd.canaryId)
	os.Exit(errCode)
	return nil
}

func patchJobCancelled(kubeclient kubernetes.Interface, ctx context.Context, jobName string, errCode int) error {
	reason:="Cancelled"
	err := patchForcefulFail(kubeclient, ctx, jobName, reason)
	if err != nil {
		return err
	}
	os.Exit(errCode)
	return nil
}

func patchJobError(kubeclient kubernetes.Interface, ctx context.Context, jobName string, errMsg string) error {
	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       errMsg,
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(kubeclient, jobStatus, ctx, jobName)
	if err != nil {
		return err
	}
	os.Exit(1)
	return nil
}

func patchForcefulFail(kubeclient kubernetes.Interface, ctx context.Context, jobName, reason string) error {
	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("The analysis was %s", reason),
				Type:          "Failed",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(kubeclient, jobStatus, ctx, jobName)
	if err != nil {
		return err
	}
	return nil
}

func patchToJob(kubeclient kubernetes.Interface, jobData JobStatus, ctx context.Context, jobName string) error {
	jsonData, err := json.Marshal(jobData)
	if err != nil {
		return err
	}

	_, err = kubeclient.BatchV1().Jobs(defaults.Namespace()).Patch(ctx, jobName, types.StrategicMergePatchType, jsonData, metav1.PatchOptions{}, "status")
	if err != nil {
		return err
	}
	return nil
}
