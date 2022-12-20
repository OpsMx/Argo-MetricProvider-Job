package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

//TODO -Retrieve the previous state?
//TODO - Rethink error

func patchJobCanaryDetails(ctx context.Context, kubeclient kubernetes.Interface, cd CanaryDetails) error {

	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("analysisDetails\n user: %s\n canaryID: %s\n reportURL: %s\n reportId: %s", cd.user, cd.canaryId, cd.reportUrl, cd.ReportId),
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(ctx, kubeclient, jobStatus, cd.jobName)
	if err != nil {
		return err
	}

	log.Infof("successfully patched to Job for canary ID %s", cd.canaryId)
	return nil
}

func patchJobSuccessful(ctx context.Context, kubeclient kubernetes.Interface, cd CanaryDetails) error {

	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("analysisDetails\n user: %s\n canaryID: %s\n reportURL: %s\n reportId: %s\n score: %s", cd.user, cd.canaryId, cd.reportUrl, cd.ReportId, cd.value),
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(ctx, kubeclient, jobStatus, cd.jobName)
	if err != nil {
		return err
	}
	log.Infof("successfully patched to Job with the score for canary ID %s", cd.canaryId)
	return nil
}

func patchJobFailedInconclusive(ctx context.Context, kubeclient kubernetes.Interface, reason string, cd CanaryDetails) error {
	jobStatus := JobStatus{
		Status: Status{
			Conditions: &[]Conditions{{
				Message:       fmt.Sprintf("analysisDetails\n user: %s\n canaryID: %s\n reportURL: %s\n reportId: %s\n score: %s", cd.user, cd.canaryId, cd.reportUrl, cd.ReportId, cd.value),
				Type:          "OpsmxAnalysis",
				LastProbeTime: metav1.NewTime(time.Now()),
				Status:        "True",
			},
			},
		},
	}
	err := patchToJob(ctx, kubeclient, jobStatus, cd.jobName)
	if err != nil {
		return err
	}
	err = patchForcefulFail(ctx, kubeclient, cd.jobName, reason)
	if err != nil {
		return err
	}
	log.Infof("successfully patched to Job with the score for canary ID %s", cd.canaryId)
	return nil
}

func patchJobCancelled(ctx context.Context, kubeclient kubernetes.Interface, jobName string) error {
	reason := "Cancelled"
	err := patchForcefulFail(ctx, kubeclient, jobName, reason)
	if err != nil {
		return err
	}
	return nil
}

func patchJobError(ctx context.Context, kubeclient kubernetes.Interface, jobName string, errMsg string) error {
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
	err := patchToJob(ctx, kubeclient, jobStatus, jobName)
	if err != nil {
		return err
	}
	return nil
}

func patchForcefulFail(ctx context.Context, kubeclient kubernetes.Interface, jobName, reason string) error {
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
	err := patchToJob(ctx, kubeclient, jobStatus, jobName)
	if err != nil {
		return err
	}
	return nil
}

func patchToJob(ctx context.Context, kubeclient kubernetes.Interface, jobData JobStatus, jobName string) error {
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
