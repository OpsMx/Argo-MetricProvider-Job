package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

//TODO - needs a massive redo

func getDummyJob(condition batchv1.JobCondition) batchv1.Job {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jobname-123",
			Namespace: "dummynamespace",
		},
		Status: batchv1.JobStatus{},
	}
	condn := condition

	job.Status.Conditions = append(job.Status.Conditions, condn)
	return job
}

func jobFakeClient(cond batchv1.JobCondition) *k8sfake.Clientset {
	job := getDummyJob(cond)
	fakeClient := k8sfake.NewSimpleClientset()
	fakeClient.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &job, nil
	})
	return fakeClient
}

func TestPatchJobCanaryDetails(t *testing.T) {
	cd := CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
	}
	cond := batchv1.JobCondition{
		Message: fmt.Sprintf("Canary ID: %s\nReport URL: %s", cd.canaryId, cd.reportUrl),
		Type:    "OpsmxAnalysis",
		Status:  "True",
	}
	k8sclient := jobFakeClient(cond)
	err := patchJobCanaryDetails(context.TODO(), k8sclient, cd)
	assert.Equal(t, nil, err)
}

func TestPatchJobSuccessful(t *testing.T) {
	cd := CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
		value:     "98",
	}
	cond := batchv1.JobCondition{
		Message: fmt.Sprintf("Canary ID: %s\nReport URL: %s\nScore: %s", cd.canaryId, cd.reportUrl, cd.value),
		Type:    "OpsmxAnalysis",
		Status:  "True",
	}
	k8sclient := jobFakeClient(cond)
	err := patchJobSuccessful(context.TODO(), k8sclient, cd)
	assert.Equal(t, nil, err)

	cd = CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
		value:     "98",
	}
	err = patchJobSuccessful(context.TODO(), getFakeClient(map[string][]byte{}), cd)
	assert.Equal(t, "jobs.batch \"jobname-123\" not found", err.Error())

}

func TestPatchJobFailedInconclusive(t *testing.T) {
	cd := CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
		value:     "70",
	}
	cond := batchv1.JobCondition{
		Message: fmt.Sprintf("Canary ID: %s\nReport URL: %s\nScore: %s", cd.canaryId, cd.reportUrl, cd.value),
		Type:    "OpsmxAnalysis",
		Status:  "True",
	}

	k8sclient := jobFakeClient(cond)
	err := patchJobFailedInconclusive(context.TODO(), k8sclient, "Failed", cd)
	assert.Equal(t, nil, err)

	cd = CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
		value:     "98",
	}
	err = patchJobSuccessful(context.TODO(), getFakeClient(map[string][]byte{}), cd)
	assert.Equal(t, "jobs.batch \"jobname-123\" not found", err.Error())
}

func TestPatchJobCancelled(t *testing.T) {
	cd := CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
	}
	cond := batchv1.JobCondition{
		Message: fmt.Sprintf("Canary ID: %s\nReport URL: %s", cd.canaryId, cd.reportUrl),
		Type:    "OpsmxAnalysis",
		Status:  "True",
	}
	k8sclient := jobFakeClient(cond)
	err := patchJobCancelled(context.TODO(), k8sclient, "jobname-123")
	assert.Equal(t, nil, err)

	cd = CanaryDetails{
		jobName:   "jobname-123",
		canaryId:  "123",
		reportUrl: "https://opsmx.test.tst/reporturl/123",
		value:     "98",
	}
	err = patchJobSuccessful(context.TODO(), getFakeClient(map[string][]byte{}), cd)
	assert.Equal(t, "jobs.batch \"jobname-123\" not found", err.Error())
}

func TestPatchJobError(t *testing.T) {
	cond := batchv1.JobCondition{
		Message:       "the error message",
		Type:          "OpsmxAnalysis",
		LastProbeTime: metav1.NewTime(time.Now()),
		Status:        "True",
	}
	k8sclient := jobFakeClient(cond)
	err := patchJobError(context.TODO(), k8sclient, "jobname-123", "the error message")
	assert.Equal(t, nil, err)
}
