package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func getFakeClient(dataParam map[string][]byte) *k8sfake.Clientset {
	data := map[string][]byte{
		"cd-integration": []byte("true"),
		"gate-url":       []byte("https://opsmx.secret.tst"),
		"source-name":    []byte("sourcename"),
		"user":           []byte("admin"),
	}
	if len(dataParam) != 0 {
		data = dataParam
	}
	opsmxSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultSecretName,
		},
		Data: data,
	}
	fakeClient := k8sfake.NewSimpleClientset()
	fakeClient.PrependReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, opsmxSecret, nil
	})

	return fakeClient
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) http.Client {
	return http.Client{
		Transport: fn,
	}
}

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFuncGetAnalysisTemplateData(t *testing.T) {
	_ = os.MkdirAll("testcases/provider", os.ModePerm)
	emptyFile, _ := os.Create("testcases/provider/providerConfig")
	emptyFile.Close()
	input, _ := os.ReadFile("testcases/analysis/providerConfig")
	_ = os.WriteFile("testcases/provider/providerConfig", input, 0644)
	metric, err := getAnalysisTemplateData("testcases/")
	checkMetric := OPSMXMetric{
		Application:     "final-job",
		User:            "admin",
		OpsmxIsdUrl:     "https://isd.opsmx.net/",
		LifetimeMinutes: 3,
		IntervalTime:    3,
		LookBackType:    "sliding",
		Pass:            80,
		Services:        []OPSMXService{},
	}
	services := OPSMXService{
		LogTemplateName:      "loggytemp",
		LogScopeVariables:    "kubernetes.pod_name",
		BaselineLogScope:     ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:       ".*{{env.LATEST_POD_HASH}}.*",
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "${namespace_key},${pod_key},${app_name}",
		BaselineMetricScope:  "argocd,{{env.STABLE_POD_HASH}},demoapp-issuegen",
		CanaryMetricScope:    "argocd,{{env.LATEST_POD_HASH}},demoapp-issuegen",
	}
	checkMetric.Services = append(checkMetric.Services, services)
	assert.Equal(t, err, nil)
	assert.Equal(t, metric, checkMetric)
	_, err = getAnalysisTemplateData("/etc/config/")
	assert.Equal(t, err.Error(), "provider config map validation error: open /etc/config/provider/providerConfig: no such file or directory\n Action Required: Provider config map has to be mounted on '/etc/config/provider' in AnalysisTemplate and must carry data element 'providerConfig'")
	input, _ = os.ReadFile("testcases/analysis/invalid")
	_ = os.WriteFile("testcases/provider/providerConfig", input, 0644)
	_, err = getAnalysisTemplateData("testcases/")
	assert.Equal(t, err.Error(), "provider config map validation error: yaml: line 8: mapping values are not allowed in this context")
	if _, err := os.Stat("testcases/provider"); !os.IsNotExist(err) {
		os.RemoveAll("testcases/provider")
	}
}

var basicChecks = []struct {
	metric  OPSMXMetric
	message string
}{
	//Test case for no lifetimeMinutes, Baseline/Canary start time
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl: "https://opsmx.test.tst",
			Application: "testapp",
			User:        "admin",
			Pass:        80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: provide either lifetimeMinutes or end time",
	},
	//Test case for no lifetimeMinutes & EndTime
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: provide either lifetimeMinutes or end time",
	},
	//Test case when end time given and baseline and canary start time not same
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-08-02T12:45:00Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: both canaryStartTime and baselineStartTime should be kept same while using endTime argument for analysis",
	},
	//Test case when lifetimeMinutes is less than 3 minutes
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   2,
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: lifetimeMinutes cannot be less than 3 minutes",
	},
	//Test case when intervalTime is less than 3 minutes
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      2,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: intervalTime cannot be less than 3 minutes",
	},
}

func TestBasicChecks(t *testing.T) {
	for _, test := range basicChecks {
		err := test.metric.basicChecks()
		assert.Equal(t, err.Error(), test.message)
	}
	metric := OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		Application:       "testapp",
		User:              "admin",
		BaselineStartTime: "2022-08-02T13:15:00Z",
		CanaryStartTime:   "2022-08-02T13:15:00Z",
		LifetimeMinutes:   30,
		Pass:              100,

		Services: []OPSMXService{
			{
				MetricScopeVariables: "job_name",
				BaselineMetricScope:  "oes-datascience-br",
				CanaryMetricScope:    "oes-datascience-cr",
				MetricTemplateName:   "metrictemplate",
			},
		},
	}
	err := metric.basicChecks()
	assert.Equal(t, err, nil)

}

var checkTimeVariables = []struct {
	metric  OPSMXMetric
	message string
}{
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-O8-02T13:15:00Z",
			LifetimeMinutes:   30,
			Pass:              100,

			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: Error in parsing canaryStartTime: parsing time \"2022-O8-02T13:15:00Z\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"O8-02T13:15:00Z\" as \"01\"",
	},
	//Test case for inappropriate time format baseline
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-O8-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   30,
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: Error in parsing baselineStartTime: parsing time \"2022-O8-02T13:15:00Z\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"O8-02T13:15:00Z\" as \"01\"",
	},
	//Test case for inappropriate time format endTime
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-O8-02T13:15:00Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: Error in parsing endTime: parsing time \"2022-O8-02T13:15:00Z\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"O8-02T13:15:00Z\" as \"01\"",
	},
	//Test case for when end time is less than start time
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-08-02T12:45:00Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "provider config map validation error: canaryStartTime cannot be greater than endTime",
	},
}

func TestGetTimeVariables(t *testing.T) {
	for _, test := range checkTimeVariables {
		err := test.metric.getTimeVariables()
		assert.Equal(t, err.Error(), test.message)
	}
	metric := OPSMXMetric{
		OpsmxIsdUrl:     "https://opsmx.test.tst",
		Application:     "testapp",
		User:            "admin",
		LifetimeMinutes: 30,
		Pass:            80,
		Services: []OPSMXService{
			{
				MetricScopeVariables: "job_name",
				BaselineMetricScope:  "oes-datascience-br",
				CanaryMetricScope:    "oes-datascience-cr",
				MetricTemplateName:   "metrictemplate",
			},
		},
	}
	err := metric.getTimeVariables()
	assert.Equal(t, err, nil)

	metric = OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		Application:       "testapp",
		BaselineStartTime: "2022-08-02T13:15:00Z",
		CanaryStartTime:   "2022-08-02T13:15:00Z",
		EndTime:           "2022-08-02T13:45:00Z",
		Pass:              80,
		Services: []OPSMXService{
			{
				MetricScopeVariables: "job_name",
				BaselineMetricScope:  "oes-datascience-br",
				CanaryMetricScope:    "oes-datascience-cr",
				MetricTemplateName:   "metrictemplate",
			},
		},
	}
	err = metric.getTimeVariables()
	assert.Equal(t, err, nil)
	assert.Equal(t, metric.LifetimeMinutes, 30)
}

func TestSecret(t *testing.T) {
	metric := OPSMXMetric{
		Application:     "final-job",
		LifetimeMinutes: 3,
		IntervalTime:    3,
		LookBackType:    "sliding",
		Pass:            80,
		Services:        []OPSMXService{},
	}
	services := OPSMXService{
		LogTemplateName:      "loggytemp",
		LogScopeVariables:    "kubernetes.pod_name",
		BaselineLogScope:     ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:       ".*{{env.LATEST_POD_HASH}}.*",
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "${namespace_key},${pod_key},${app_name}",
		BaselineMetricScope:  "argocd,{{env.STABLE_POD_HASH}},demoapp-issuegen",
		CanaryMetricScope:    "argocd,{{env.LATEST_POD_HASH}},demoapp-issuegen",
	}
	metric.Services = append(metric.Services, services)
	_, err := metric.getDataSecret("testcases/")
	assert.Equal(t, err.Error(), "opsmx profile secret validation error: open testcases/secrets/user: no such file or directory\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'user'")
	_ = os.MkdirAll("testcases/secrets", os.ModePerm)
	emptyFile, _ := os.Create("testcases/secrets/user")
	emptyFile.Close()
	input, _ := os.ReadFile("testcases/secret/user")
	_ = os.WriteFile("testcases/secrets/user", input, 0644)

	_, err = metric.getDataSecret("testcases/")
	assert.Equal(t, err.Error(), "opsmx profile secret validation error: open testcases/secrets/opsmxIsdUrl: no such file or directory\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'opsmxIsdUrl'")
	emptyFile, _ = os.Create("testcases/secrets/opsmxIsdUrl")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/gate-url")
	_ = os.WriteFile("testcases/secrets/opsmxIsdUrl", input, 0644)

	_, err = metric.getDataSecret("testcases/")
	assert.Equal(t, err.Error(), "opsmx profile secret validation error: open testcases/secrets/sourceName: no such file or directory\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'sourceName'")
	emptyFile, _ = os.Create("testcases/secrets/sourceName")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/source-name")
	_ = os.WriteFile("testcases/secrets/sourceName", input, 0644)

	_, err = metric.getDataSecret("testcases/")
	assert.Equal(t, err.Error(), "opsmx profile secret validation error: open testcases/secrets/cdIntegration: no such file or directory\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'cdIntegration'")
	emptyFile, _ = os.Create("testcases/secrets/cdIntegration")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/cd-Integration")
	_ = os.WriteFile("testcases/secrets/cdIntegration", input, 0644)

	secretData, err := metric.getDataSecret("testcases/")
	assert.Equal(t, nil, err)
	checkSecretData := map[string]string{
		"cdIntegration": "argocd",
		"sourceName":    "argocd06",
		"opsmxIsdUrl":   "www.opsmx.com",
		"user":          "admins",
	}
	assert.Equal(t, checkSecretData, secretData)

	input, _ = os.ReadFile("testcases/secret/cd-Integration-False")
	_ = os.WriteFile("testcases/secrets/cdIntegration", input, 0644)
	secretData, err = metric.getDataSecret("testcases/")
	assert.Equal(t, err, nil)
	checkSecretData = map[string]string{
		"cdIntegration": "argorollouts",
		"sourceName":    "argocd06",
		"opsmxIsdUrl":   "www.opsmx.com",
		"user":          "admins",
	}
	assert.Equal(t, checkSecretData, secretData)

	input, _ = os.ReadFile("testcases/secret/cd-Integration-Invalid")
	_ = os.WriteFile("testcases/secrets/cdIntegration", input, 0644)
	_, err = metric.getDataSecret("testcases/")
	assert.Equal(t, err.Error(), "opsmx profile secret validation error: cdIntegration should be either true or false")
	if _, err := os.Stat("testcases/secrets"); !os.IsNotExist(err) {
		os.RemoveAll("testcases/secrets")
	}
}

var successfulPayload = []struct {
	metric                OPSMXMetric
	payloadRegisterCanary string
}{
	//Test case for basic function of Single Service feature
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			LifetimeMinutes:   30,
			IntervalTime:      3,
			Delay:             1,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables:  "job_name",
					BaselineMetricScope:   "oes-datascience-br",
					CanaryMetricScope:     "oes-datascience-cr",
					MetricTemplateName:    "metricTemplate",
					MetricTemplateVersion: "1",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
							"lifetimeMinutes": "30",
							"lookBackType": "growing",
							"interval": "3",
							"delay": "1",
							"canaryHealthCheckHandler": {
											"minimumCanaryResultScore": "80"
											},
							"canarySuccessCriteria": {
										"canaryResultScore": "80"
											}
							},
					"canaryDeployments": [
								{
								"canaryStartTimeMs": "1660137300000",
								"baselineStartTimeMs": "1660137300000",
								"canary": {
									"metric": {"service1":{"job_name":"oes-datascience-cr","serviceGate":"gate1","template":"metricTemplate","templateVersion":"1"}
								  }},
								"baseline": {
									"metric": {"service1":{"job_name":"oes-datascience-br","serviceGate":"gate1","template":"metricTemplate","templateVersion":"1"}}
								  }
								}
					  ]
				}`,
	},
	//Test case for endtime function of Single Service feature
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables:  "job_name",
					BaselineMetricScope:   "oes-datascience-br",
					CanaryMetricScope:     "oes-datascience-cr",
					MetricTemplateName:    "metricTemplate",
					MetricTemplateVersion: "1",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
							"lifetimeMinutes": "30",
							"canaryHealthCheckHandler": {
											"minimumCanaryResultScore": "80"
											},
							"canarySuccessCriteria": {
										"canaryResultScore": "80"
											}
							},
					"canaryDeployments": [
								{
								"canaryStartTimeMs": "1660137300000",
								"baselineStartTimeMs": "1660137300000",
								"canary": {
									"metric": {"service1":{"job_name":"oes-datascience-cr","serviceGate":"gate1","template":"metricTemplate","templateVersion":"1"}
								  }},
								"baseline": {
									"metric": {"service1":{"job_name":"oes-datascience-br","serviceGate":"gate1","template":"metricTemplate","templateVersion":"1"}}
								  }
								}
					  ]
				}`,
	},
	//Test case for only 1 time stamp given function of Single Service feature
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables:  "job_name",
					BaselineMetricScope:   "oes-datascience-br",
					CanaryMetricScope:     "oes-datascience-cr",
					MetricTemplateName:    "metricTemplate",
					MetricTemplateVersion: "1",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
							"lifetimeMinutes": "30",
							"canaryHealthCheckHandler": {
											"minimumCanaryResultScore": "80"
											},
							"canarySuccessCriteria": {
										"canaryResultScore": "80"
											}
							},
					"canaryDeployments": [
								{
								"canaryStartTimeMs": "1660137300000",
								"baselineStartTimeMs": "1660137300000",
								"canary": {
									"metric": {"service1":{"job_name":"oes-datascience-cr","serviceGate":"gate1","template":"metricTemplate","templateVersion":"1"}
								  }},
								"baseline": {
									"metric": {"service1":{"job_name":"oes-datascience-br","serviceGate":"gate1","template":"metricTemplate","templateVersion":"1"}}
								  }
								}
					  ]
				}`,
	},
	//Test case for multi-service feature
	{
		metric: OPSMXMetric{
			User:                 "admin",
			OpsmxIsdUrl:          "https://opsmx.test.tst",
			Application:          "multiservice",
			BaselineStartTime:    "2022-08-10T13:15:00Z",
			CanaryStartTime:      "2022-08-10T13:15:00Z",
			EndTime:              "2022-08-10T13:45:10Z",
			GlobalMetricTemplate: "metricTemplate",
			Pass:                 80,
			Services: []OPSMXService{
				{
					MetricScopeVariables:  "job_name",
					BaselineMetricScope:   "oes-sapor-br",
					CanaryMetricScope:     "oes-sapor-cr",
					MetricTemplateName:    "metricTemplate",
					MetricTemplateVersion: "1",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
				},
			},
		},
		payloadRegisterCanary: `		{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
						"lifetimeMinutes": "30",
					  "canaryHealthCheckHandler": {
						"minimumCanaryResultScore": "80"
					  },
					  "canarySuccessCriteria": {
						"canaryResultScore": "80"
					  }
					},
					"canaryDeployments": [
					  {
						"canaryStartTimeMs": "1660137300000",
						"baselineStartTimeMs": "1660137300000",
						"canary": {
						  "metric": {
							"service1": {
							  "job_name": "oes-sapor-cr",
							  "serviceGate": "gate1",
							  "template":"metricTemplate",
							  "templateVersion":"1"
							},
							"service2": {
							  "job_name": "oes-platform-cr",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						},
						"baseline": {
						  "metric": {
							"service1": {
							  "job_name": "oes-sapor-br",
							  "serviceGate": "gate1",
							  "template":"metricTemplate",
							  "templateVersion":"1"
							},
							"service2": {
							  "job_name": "oes-platform-br",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						}
					  }
					]
				  }`,
	},

	//Test case for multi-service feature along with logs+metrics analysis
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables:  "job_name",
					BaselineMetricScope:   "oes-platform-br",
					CanaryMetricScope:     "oes-platform-cr",
					MetricTemplateName:    "metricTemplate",
					MetricTemplateVersion: "1",
				},
				{
					MetricScopeVariables:  "job_name",
					BaselineMetricScope:   "oes-sapor-br",
					CanaryMetricScope:     "oes-sapor-cr",
					MetricTemplateName:    "metricTemplate",
					MetricTemplateVersion: "1",
					LogScopeVariables:     "kubernetes.container_name",
					BaselineLogScope:      "oes-datascience-br",
					CanaryLogScope:        "oes-datascience-cr",
					LogTemplateName:       "logTemplate",
					LogTemplateVersion:    "1",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
					  "lifetimeMinutes": "30",
					  "canaryHealthCheckHandler": {
						"minimumCanaryResultScore": "80"
					  },
					  "canarySuccessCriteria": {
						"canaryResultScore": "80"
					  }
					},
					"canaryDeployments": [
					  {
						"canaryStartTimeMs": "1660137300000",
						"baselineStartTimeMs": "1660137300000",
						"canary": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-cr",
							  "serviceGate": "gate2",
							  "template":"logTemplate",
							  "templateVersion":"1"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-cr",
							  "serviceGate": "gate1",
							  "template":"metricTemplate",
							  "templateVersion":"1"
							},
							"service2": {
							  "job_name": "oes-sapor-cr",
							  "serviceGate": "gate2",
							  "template":"metricTemplate",
							  "templateVersion":"1"
							}
						  }
						},
						"baseline": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-br",
							  "serviceGate": "gate2",
							  "template":"logTemplate",
							  "templateVersion":"1"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-br",
							  "serviceGate": "gate1",
							  "template":"metricTemplate",
							  "templateVersion":"1"
							},
							"service2": {
							  "job_name": "oes-sapor-br",
							  "serviceGate": "gate2",
							  "template":"metricTemplate",
							  "templateVersion":"1"
							}
						  }
						}
					  }
					]
				  }`,
	},
	//Test case for 1 incorrect service and one correct
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metricTemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metricTemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logTemplate",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
						"lifetimeMinutes": "30",
					  "canaryHealthCheckHandler": {
						"minimumCanaryResultScore": "80"
					  },
					  "canarySuccessCriteria": {
						"canaryResultScore": "80"
					  }
					},
					"canaryDeployments": [
					  {
						"canaryStartTimeMs": "1660137300000",
						"baselineStartTimeMs": "1660137300000",
						"canary": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-cr",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-cr",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-cr",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						},
						"baseline": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-br",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-br",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-br",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						}
					  }
					]
				  }`,
	},
	//Test case for Service Name given
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					ServiceName:          "service1",
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metricTemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metricTemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logTemplate",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
						"lifetimeMinutes": "30",
					  "canaryHealthCheckHandler": {
						"minimumCanaryResultScore": "80"
					  },
					  "canarySuccessCriteria": {
						"canaryResultScore": "80"
					  }
					},
					"canaryDeployments": [
					  {
						"canaryStartTimeMs": "1660137300000",
						"baselineStartTimeMs": "1660137300000",
						"canary": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-cr",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-cr",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-cr",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						},
						"baseline": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-br",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-br",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-br",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						}
					  }
					]
				  }`,
	},
	//Test case for Global log Template
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			GlobalLogTemplate: "logTemplate",
			Pass:              80,
			Services: []OPSMXService{
				{
					ServiceName:          "service1",
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metricTemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metricTemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",
					"canaryConfig": {
						"lifetimeMinutes": "30",
					  "canaryHealthCheckHandler": {
						"minimumCanaryResultScore": "80"
					  },
					  "canarySuccessCriteria": {
						"canaryResultScore": "80"
					  }
					},
					"canaryDeployments": [
					  {
						"canaryStartTimeMs": "1660137300000",
						"baselineStartTimeMs": "1660137300000",
						"canary": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-cr",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-cr",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-cr",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						},
						"baseline": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-br",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  
							  "job_name": "oes-platform-br",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-br",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						}
					  }
					]
				  }`,
	},
	//Test case for CanaryStartTime not given but baseline was given
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					ServiceName:          "service1",
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metricTemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metricTemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logTemplate",
				},
			},
		},
		payloadRegisterCanary: `{
					"application": "multiservice",
					"sourceName":"sourcename",
					"sourceType":"argocd",	
					"canaryConfig": {
						"lifetimeMinutes": "30",
					  "canaryHealthCheckHandler": {
						"minimumCanaryResultScore": "80"
					  },
					  "canarySuccessCriteria": {
						"canaryResultScore": "80"
					  }
					},
					"canaryDeployments": [
					  {
						"canaryStartTimeMs": "1660137300000",
						"baselineStartTimeMs": "1660137300000",
						"canary": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-cr",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-cr",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-cr",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						},
						"baseline": {
						  "log": {
							"service2": {
							  "kubernetes.container_name": "oes-datascience-br",
							  "serviceGate": "gate2",
							  "template":"logTemplate"
							}
						  },
						  "metric": {
							"service1": {
							  "job_name": "oes-platform-br",
							  "serviceGate": "gate1",
							  "template":"metricTemplate"
							},
							"service2": {
							  "job_name": "oes-sapor-br",
							  "serviceGate": "gate2",
							  "template":"metricTemplate"
							}
						  }
						}
					  }
					]
				  }`,
	},
}
var failPayload = []struct {
	metric  OPSMXMetric
	message string
}{
	//Test case for No log & Metric analysis
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
		},
		message: "provider config map validation error: no services provided",
	},
	//Test case for No log & Metric analysis
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					ServiceName: "service1",
				},
				{
					ServiceName: "service2",
				},
			},
		},
		message: "provider config map validation error: at least one of log or metric context must be provided",
	},
	//Test case for mismatch in log scope variables and baseline/canary log scope
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metrictemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "kubernetes.container_name,kubernetes.pod",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemplate",
				},
			},
		},
		message: "provider config map validation error: mismatch in number of log scope variables and baseline/canary log scope of service 'service2'",
	},

	//Test case for mismatch in metric scope variables and baseline/canary metric scope
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name,job123",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metrictemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemplate",
				},
			},
		},
		message: "provider config map validation error: mismatch in number of metric scope variables and baseline/canary metric scope of service 'service1'",
	},
	//Test case when baseline or canary logplaceholder is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-cr",
					LogTemplateName:      "logtemplate",
				},
			},
		},
		message: "provider config map validation error: missing canary for log analysis of service 'service1'",
	},

	//Test case when baseline or canary metricplaceholder is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascienece-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemplate",
				},
			},
		},
		message: "provider config map validation error: missing baseline/canary for metric analysis of service 'service1'",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascienece-br",
					CanaryLogScope:       "oes-datascience-cr",
				},
			},
		},
		message: "provider config map validation error: provide either a service specific log template or global log template for service 'service1'",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascienece-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemp",
				},
			},
		},
		message: "provider config map validation error: provide either a service specific metric template or global metric template for service: service1",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					BaselineLogScope:     "oes-datascienece-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemp",
				},
			},
		},
		message: "provider config map validation error: missing log Scope placeholder for the provided baseline/canary of service 'service1'",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemp",
				},
			},
		},
		message: "provider config map validation error: missing log Scope placeholder for the provided baseline/canary of service 'service1'",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					BaselineLogScope:     "oes-datascienece-br",
					LogTemplateName:      "logtemp",
				},
			},
		},
		message: "provider config map validation error: missing log Scope placeholder for the provided baseline/canary of service 'service1'",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					BaselineMetricScope: "oes-sapor-br",
					CanaryMetricScope:   "oes-sapor-cr",
					LogScopeVariables:   "kubernetes.container_name",
					BaselineLogScope:    "oes-datascienece-br",
					CanaryLogScope:      "oes-datascience-cr",
					LogTemplateName:     "logtemp",
				},
			},
		},
		message: "provider config map validation error: missing metric Scope placeholder for the provided baseline/canary of service 'service1'",
	},

	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					CanaryMetricScope: "oes-sapor-cr",
					LogScopeVariables: "kubernetes.container_name",
					BaselineLogScope:  "oes-datascienece-br",
					CanaryLogScope:    "oes-datascience-cr",
					LogTemplateName:   "logtemp",
				},
			},
		},
		message: "provider config map validation error: missing metric Scope placeholder for the provided baseline/canary of service 'service1'",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			OpsmxIsdUrl:       "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Services: []OPSMXService{
				{
					BaselineMetricScope: "oes-sapor-br",
					LogScopeVariables:   "kubernetes.container_name",
					BaselineLogScope:    "oes-datascienece-br",
					CanaryLogScope:      "oes-datascience-cr",
					LogTemplateName:     "logtemp",
				},
			},
		},
		message: "provider config map validation error: missing metric Scope placeholder for the provided baseline/canary of service 'service1'",
	},
}

func TestPayload(t *testing.T) {
	httpclient := NewHttpClient()
	clients := newClients(nil, httpclient)
	SecretData := map[string]string{
		"cdIntegration": "argocd",
		"sourceName":    "sourcename",
		"opsmxIsdUrl":   "www.opsmx.com",
		"user":          "admins",
	}

	for _, test := range successfulPayload {
		err := test.metric.getTimeVariables()
		assert.Equal(t, nil, err)
		payload, err := test.metric.generatePayload(clients, SecretData, "notrequired")
		assert.Equal(t, nil, err)
		processedPayload := strings.Replace(strings.Replace(strings.Replace(test.payloadRegisterCanary, "\n", "", -1), "\t", "", -1), " ", "", -1)
		assert.Equal(t, processedPayload, payload)
	}
	for _, test := range failPayload {
		err := test.metric.getTimeVariables()
		assert.Equal(t, nil, err)
		_, err = test.metric.generatePayload(clients, SecretData, "notrequired")
		assert.Equal(t, test.message, err.Error())
	}
	metric := OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		User:              "admin",
		Application:       "multiservice",
		BaselineStartTime: "2022-08-10T13:15:00Z",
		CanaryStartTime:   "2022-08-10T13:15:00Z",
		LifetimeMinutes:   30,
		IntervalTime:      3,
		Delay:             1,
		LookBackType:      "growing",
		Pass:              80,
		Services:          []OPSMXService{},
	}
	services := OPSMXService{
		LogTemplateName:      "loggytemp",
		LogScopeVariables:    "kubernetes.pod_name",
		BaselineLogScope:     ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:       ".*{{env.LATEST_POD_HASH}}.*",
		MetricTemplateName:   "metrix",
		MetricScopeVariables: "kubernetes.pod_name",
		BaselineMetricScope:  ".*{{env.STABLE_POD_METRIC_HASH}}.*",
		CanaryMetricScope:    ".*{{env.LATEST_POD_METRIC_HASH}}.*",
	}
	metric.Services = append(metric.Services, services)
	err := metric.getTimeVariables()
	assert.Equal(t, nil, err)
	_, err = metric.generatePayload(clients, SecretData, "notRequired")
	assert.Equal(t, "analysisTemplate validation error: environment variable STABLE_POD_HASH not set", err.Error())
	os.Setenv("STABLE_POD_HASH", "baseline")
	_, err = metric.generatePayload(clients, SecretData, "notRequired")
	assert.Equal(t, "analysisTemplate validation error: environment variable LATEST_POD_HASH not set", err.Error())
	os.Setenv("LATEST_POD_HASH", "baseline")
	_, err = metric.generatePayload(clients, SecretData, "notRequired")
	assert.Equal(t, "analysisTemplate validation error: environment variable STABLE_POD_METRIC_HASH not set", err.Error())
	os.Setenv("STABLE_POD_METRIC_HASH", "baseline")
	_, err = metric.generatePayload(clients, SecretData, "notRequired")
	assert.Equal(t, "analysisTemplate validation error: environment variable LATEST_POD_METRIC_HASH not set", err.Error())
}

func TestGitops(t *testing.T) {
	os.Setenv("STABLE_POD_HASH", "baseline")
	os.Setenv("LATEST_POD_HASH", "canary")
	SecretData := map[string]string{
		"cdIntegration": "argocd",
		"sourceName":    "sourcename",
		"opsmxIsdUrl":   "http://www.opsmx.com",
		"user":          "admins",
	}
	metric := OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		User:              "admin",
		Application:       "multiservice",
		BaselineStartTime: "2022-08-10T13:15:00Z",
		CanaryStartTime:   "2022-08-10T13:15:00Z",
		LifetimeMinutes:   30,
		IntervalTime:      3,
		Delay:             1,
		GitOPS:            true,
		LookBackType:      "growing",
		Pass:              80,
		Services:          []OPSMXService{},
	}
	services := OPSMXService{
		LogTemplateName:   "loggytemp",
		LogScopeVariables: "kubernetes.pod_name",
		BaselineLogScope:  ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:    ".*{{env.LATEST_POD_HASH}}.*",
	}
	checkPayload := `{
		"application": "multiservice",
		"sourceName":"sourcename",
		"sourceType":"argocd",
		"canaryConfig": {
				"lifetimeMinutes": "30",
				"lookBackType": "growing",
				"interval": "3",
				"delay": "1",
				"canaryHealthCheckHandler": {
								"minimumCanaryResultScore": "80"
								},
				"canarySuccessCriteria": {
							"canaryResultScore": "80"
								}
				},
		"canaryDeployments": [
					{
					"canaryStartTimeMs": "1660137300000",
					"baselineStartTimeMs": "1660137300000",
					"canary": {
						"log": {"service1":{
						"kubernetes.pod_name":".*canary.*",
						"serviceGate":"gate1",
						"template":"loggytemp",
						"templateSha1":"1fd53480333cb618aa05ce901a051263efabe3cd"}
					  }},
					"baseline": {
						"log": {"service1":{
						"kubernetes.pod_name":".*baseline.*",
						"serviceGate":"gate1",
						"template":"loggytemp",
						"templateSha1":"1fd53480333cb618aa05ce901a051263efabe3cd"}}
					  }
					}
		  ]
	}`
	c := NewTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == "POST" {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"timestamp": 1662442034995,
					"status": 200,
					"error": "CREATED",
					"errorMessage": []
				  }
				`)),
				Header: make(http.Header),
			}, nil
		} else {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`false`)),
				Header:     make(http.Header),
			}, nil
		}
	})
	clients := newClients(nil, c)
	metric.Services = append(metric.Services, services)
	err := metric.getTimeVariables()
	assert.Equal(t, nil, err)
	_, err = metric.generatePayload(clients, SecretData, "incorrect/")
	assert.Equal(t, "gitops 'loggytemp' template config map validation error: open incorrect/templates/loggytemp: no such file or directory\n Action Required: Template has to be mounted on '/etc/config/templates' in AnalysisTemplate and must carry data element 'loggytemp'", err.Error())

	_ = os.MkdirAll("testcases/templates", os.ModePerm)
	emptyFile, _ := os.Create("testcases/templates/loggytemp")
	emptyFile.Close()
	input, _ := os.ReadFile("testcases/gitops/loggytemp")
	_ = os.WriteFile("testcases/templates/loggytemp", input, 0644)
	payload, err := metric.generatePayload(clients, SecretData, "testcases/")
	assert.Equal(t, nil, err)
	processedPayload := strings.Replace(strings.Replace(strings.Replace(checkPayload, "\n", "", -1), "\t", "", -1), " ", "", -1)
	assert.Equal(t, processedPayload, payload)

	metric = OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		User:              "admin",
		Application:       "multiservice",
		BaselineStartTime: "2022-08-10T13:15:00Z",
		CanaryStartTime:   "2022-08-10T13:15:00Z",
		LifetimeMinutes:   30,
		IntervalTime:      3,
		Delay:             1,
		GitOPS:            true,
		LookBackType:      "growing",
		Pass:              80,
		Services:          []OPSMXService{},
	}
	services = OPSMXService{
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "kubernetes.pod_name",
		BaselineMetricScope:  ".*{{env.STABLE_POD_HASH}}.*",
		CanaryMetricScope:    ".*{{env.LATEST_POD_HASH}}.*",
	}
	checkPayload = `{
		"application": "multiservice",
		"sourceName":"sourcename",
		"sourceType":"argocd",
		"canaryConfig": {
				"lifetimeMinutes": "30",
				"lookBackType": "growing",
				"interval": "3",
				"delay": "1",
				"canaryHealthCheckHandler": {
								"minimumCanaryResultScore": "80"
								},
				"canarySuccessCriteria": {
							"canaryResultScore": "80"
								}
				},
		"canaryDeployments": [
					{
					"canaryStartTimeMs": "1660137300000",
					"baselineStartTimeMs": "1660137300000",
					"canary": {
						"metric": {"service1":{
						"kubernetes.pod_name":".*canary.*",
						"serviceGate":"gate1",
						"template":"PrometheusMetricTemplate",
						"templateSha1":"445b4c60855cd618b070e91ee232860e40e23d9c"}
					  }},
					"baseline": {
						"metric": {"service1":{
						"kubernetes.pod_name":".*baseline.*",
						"serviceGate":"gate1",
						"template":"PrometheusMetricTemplate",
						"templateSha1":"445b4c60855cd618b070e91ee232860e40e23d9c"}}
					  }
					}
		  ]
	}`

	metric.Services = append(metric.Services, services)
	err = metric.getTimeVariables()
	assert.Equal(t, nil, err)
	emptyFile, _ = os.Create("testcases/templates/PrometheusMetricTemplate")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/gitops/PrometheusMetricTemplate")
	_ = os.WriteFile("testcases/templates/PrometheusMetricTemplate", input, 0644)
	_, err = metric.generatePayload(clients, SecretData, "gitops/nothere/")
	assert.Equal(t, "gitops 'PrometheusMetricTemplate' template config map validation error: open gitops/nothere/templates/PrometheusMetricTemplate: no such file or directory\n Action Required: Template has to be mounted on '/etc/config/templates' in AnalysisTemplate and must carry data element 'PrometheusMetricTemplate'", err.Error())
	payload, err = metric.generatePayload(clients, SecretData, "testcases/")
	assert.Equal(t, nil, err)
	processedPayload = strings.Replace(strings.Replace(strings.Replace(checkPayload, "\n", "", -1), "\t", "", -1), " ", "", -1)
	assert.Equal(t, processedPayload, payload)

	metric = OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		User:              "admin",
		Application:       "multiservice",
		BaselineStartTime: "2022-08-10T13:15:00Z",
		CanaryStartTime:   "2022-08-10T13:15:00Z",
		LifetimeMinutes:   30,
		IntervalTime:      3,
		Delay:             1,
		GitOPS:            true,
		LookBackType:      "growing",
		Pass:              80,
		Services:          []OPSMXService{},
	}
	services = OPSMXService{
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "kubernetes.pod_name",
		BaselineMetricScope:  ".*{{env.STABLE_POD_HASH}}.*",
		CanaryMetricScope:    ".*{{env.LATEST_POD_HASH}}.*",
	}
	c = NewTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == "POST" {
			return &http.Response{
				StatusCode: 500,
				Body: io.NopCloser(bytes.NewBufferString(`
			{
				"timestamp": 1662442034995,
				"status": 500,
				"error": "Internal Server Error",
				"errorMessage": [
				  "ISD-EmptyKeyOrValueInJson-400-07 : Analytics Service - Name key or value is missing in json !",
				  "ISD-EmptyKeyOrValueInJson-400-07 : Analytics Service - Account name key or value is missing in json !",
				  "ISD-IsNotFound-404-01 : Analytics Service - Datasource account not found : "
				],
				"exception": "feign.FeignException$NotFound",
				"message": "Template Already Exists"
			  }
			`)),
				Header: make(http.Header),
			}, nil
		} else {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`false`)),
				Header:     make(http.Header),
			}, nil
		}
	})
	clientFail := newClients(nil, c)
	metric.Services = append(metric.Services, services)
	err = metric.getTimeVariables()
	assert.Equal(t, nil, err)
	_, err = getTemplateData(clientFail.client, SecretData, "loggytemp", "LOG", "testcases/", "scope")
	assert.Equal(t, "gitops 'loggytemp' template config map validation error: ISD-EmptyKeyOrValueInJson-400-07 : Analytics Service - Name key or value is missing in json ! ISD-EmptyKeyOrValueInJson-400-07 : Analytics Service - Account name key or value is missing in json ! ISD-IsNotFound-404-01 : Analytics Service - Datasource account not found : ", err.Error())

	invalidjsonmetric := OPSMXMetric{
		Application:     "final-job",
		LifetimeMinutes: 3,
		IntervalTime:    3,
		LookBackType:    "sliding",
		Pass:            80,
		GitOPS:          true,
		Services:        []OPSMXService{},
	}
	invalidjsonservices := OPSMXService{
		LogTemplateName:      "invalid.txt",
		LogScopeVariables:    "kubernetes.pod_name",
		BaselineLogScope:     ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:       ".*{{env.LATEST_POD_HASH}}.*",
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "${namespace_key},${pod_key},${app_name}",
		BaselineMetricScope:  "argocd,{{env.STABLE_POD_HASH}},demoapp-issuegen",
		CanaryMetricScope:    "argocd,{{env.LATEST_POD_HASH}},demoapp-issuegen",
	}
	invalidjsonmetric.Services = append(invalidjsonmetric.Services, invalidjsonservices)
	emptyFile, _ = os.Create("testcases/templates/invalid.txt")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/gitops/invalid/loggytemp.txt")
	_ = os.WriteFile("testcases/templates/invalid.txt", input, 0644)
	_, err = invalidjsonmetric.generatePayload(clients, SecretData, "testcases/")
	assert.Equal(t, "gitops 'invalid.txt' template config map validation error: yaml: line 22: did not find expected ',' or '}'", err.Error())

	metric = OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		User:              "admin",
		Application:       "multiservice",
		BaselineStartTime: "2022-08-10T13:15:00Z",
		CanaryStartTime:   "2022-08-10T13:15:00Z",
		LifetimeMinutes:   30,
		IntervalTime:      3,
		Delay:             1,
		GitOPS:            true,
		LookBackType:      "growing",
		Pass:              80,
		Services:          []OPSMXService{},
	}
	services = OPSMXService{
		LogTemplateName:   "loggytemp",
		LogScopeVariables: "kubernetes.pod_name",
		BaselineLogScope:  ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:    ".*{{env.LATEST_POD_HASH}}.*",
	}
	metric.Services = append(metric.Services, services)
	cinv := NewTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == "POST" {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"status": 200,
					"error": "CREATED",
					"errorMessage": []
				  }
			`)),
				Header: make(http.Header),
			}, nil
		} else {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{false}`)),
				Header:     make(http.Header),
			}, nil
		}
	})
	clientInvalid := newClients(nil, cinv)
	metric.Services = append(metric.Services, services)
	err = metric.getTimeVariables()
	assert.Equal(t, nil, err)
	_, err = getTemplateData(clientInvalid.client, SecretData, "loggytemp", "LOG", "testcases/", "scope")
	assert.Equal(t, "analysis Error: Expected bool response from gitops verifyTemplate response  Error: invalid character 'f' looking for beginning of object key string. Action: Check endpoint given in secret/providerConfig.", err.Error())

	cinv = NewTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == "POST" {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"status" 200,
					"error": "CREATED",
					"errorMessage": []
				  }
			`)),
				Header: make(http.Header),
			}, nil
		} else {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`false`)),
				Header:     make(http.Header),
			}, nil
		}
	})
	clientInvalid = newClients(nil, cinv)
	_, err = getTemplateData(clientInvalid.client, SecretData, "loggytemp", "LOG", "testcases/", "scope")
	assert.Equal(t, "invalid character '2' after object key", err.Error())
	if _, err := os.Stat("testcases/templates"); !os.IsNotExist(err) {
		os.RemoveAll("testcases/templates")
	}

}

func TestProcessResume(t *testing.T) {
	metric := OPSMXMetric{
		OpsmxIsdUrl:       "https://opsmx.test.tst",
		User:              "admin",
		Application:       "multiservice",
		BaselineStartTime: "2022-08-10T13:15:00Z",
		CanaryStartTime:   "2022-08-10T13:15:00Z",
		LifetimeMinutes:   30,
		IntervalTime:      3,
		Delay:             1,
		GitOPS:            true,
		LookBackType:      "growing",
		Pass:              80,
		Services:          []OPSMXService{},
	}
	services := OPSMXService{
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "kubernetes.pod_name",
		BaselineMetricScope:  ".*{{env.STABLE_POD_HASH}}.*",
		CanaryMetricScope:    ".*{{env.LATEST_POD_HASH}}.*",
	}
	metric.Services = append(metric.Services, services)
	input, _ := io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds",
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"overallScore": 100,
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	phase, canaryScore, err := metric.processResume(input)
	assert.Equal(t, nil, err)
	assert.Equal(t, "100", canaryScore)
	assert.Equal(t, AnalysisPhaseSuccessful, phase)

	input, _ = io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds",
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"overallScore": 0,
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	phase, canaryScore, err = metric.processResume(input)
	assert.Equal(t, nil, err)
	assert.Equal(t, "0", canaryScore)
	assert.Equal(t, AnalysisPhaseFailed, phase)

	input, _ = io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds",
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	phase, canaryScore, err = metric.processResume(input)
	assert.Equal(t, nil, err)
	assert.Equal(t, "0", canaryScore)
	assert.Equal(t, AnalysisPhaseFailed, phase)

	input, _ = io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds",
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"overallScore": 97.25,
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	phase, canaryScore, err = metric.processResume(input)
	assert.Equal(t, nil, err)
	assert.Equal(t, "97", canaryScore)
	assert.Equal(t, AnalysisPhaseSuccessful, phase)

	input, _ = io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds",
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"overallScore": "97.2a5",
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	_, _, err = metric.processResume(input)

	assert.Equal(t, "strconv.ParseFloat: parsing \"97.2a5\": invalid syntax", err.Error())

	input, _ = io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds",
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"overallScore": "9a7",
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	_, _, err = metric.processResume(input)

	assert.Equal(t, "strconv.Atoi: parsing \"9a7\": invalid syntax", err.Error())

	input, _ = io.ReadAll(bytes.NewBufferString(`
	{
		"owner": "admin",
		"application": "testapp",
		"canaryResult": {
			"duration": "0 seconds"
			"lastUpdated": "2022-09-02 10:02:18.504",
			"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
			"overallScore": "97",
			"intervalNo": 1,
			"isLastRun": true,
			"overallResult": "HEALTHY",
			"message": "Canary Is HEALTHY",
			"errors": []
		},
		"launchedDate": "2022-09-02 10:02:18.504",
		"canaryConfig": {
			"combinedCanaryResultStrategy": "LOWEST",
			"minimumCanaryResultScore": 65.0,
			"name": "admin",
			"lifetimeMinutes": 30,
			"canaryAnalysisIntervalMins": 30,
			"maximumCanaryResultScore": 80.0
		},
		"id": "1424",
		"services": [],
		"status": {
			"complete": false,
			"status": "COMPLETED"
		}}
	`))
	_, _, err = metric.processResume(input)

	assert.Equal(t, "analysis Error: Error in post processing canary Response. Error: invalid character '\"' after object key:value pair", err.Error())
}

func TestRunAnalysis(t *testing.T) {
	os.Setenv("STABLE_POD_HASH", "baseline")
	os.Setenv("LATEST_POD_HASH", "canary")
	Head := map[string][]string{
		"Location": {"www.scoreUrl.com"},
	}
	c := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(bytes.NewBufferString(`
			{
				"canaryId": 1424
			}
			`)),
			// Must be set to non-nil value or it panics
			Header: Head,
		}, nil
	})
	resourceNames := ResourceNames{
		podName: "podName",
		jobName: "jobname-123",
	}
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
	clients := newClients(k8sclient, c)
	_ = os.MkdirAll("testcases/secrets", os.ModePerm)
	_ = os.MkdirAll("testcases/provider", os.ModePerm)
	emptyFile, _ := os.Create("testcases/secrets/user")
	emptyFile.Close()
	input, _ := os.ReadFile("testcases/secret/user")
	_ = os.WriteFile("testcases/secrets/user", input, 0644)
	emptyFile, _ = os.Create("testcases/secrets/opsmxIsdUrl")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/gate-url")
	_ = os.WriteFile("testcases/secrets/opsmxIsdUrl", input, 0644)
	emptyFile, _ = os.Create("testcases/secrets/cdIntegration")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/cd-Integration")
	_ = os.WriteFile("testcases/secrets/cdIntegration", input, 0644)
	emptyFile, _ = os.Create("testcases/secrets/sourceName")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/source-name")
	_ = os.WriteFile("testcases/secrets/sourceName", input, 0644)
	emptyFile, _ = os.Create("testcases/provider/providerConfig")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/analysis/providerConfig")
	_ = os.WriteFile("testcases/provider/providerConfig", input, 0644)
	_, err := runAnalysis(clients, resourceNames, "testcases/")
	assert.Equal(t, nil, err)

	cInv := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(bytes.NewBufferString(`
			{
				canaryId: 1424
			}
			`)),
			// Must be set to non-nil value or it panics
			Header: Head,
		}, nil
	})
	clientsInv := newClients(k8sclient, cInv)
	_, err = runAnalysis(clientsInv, resourceNames, "testcases/")
	assert.Equal(t, `invalid character 'c' looking for beginning of object key string`, err.Error())

	cInv = NewTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == "POST" {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
			{
				"canaryId": 1424
			}
			`)),
				// Must be set to non-nil value or it panics
				Header: Head,
			}, nil
		} else {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					canaryId: 1424
				}
				`)),
				// Must be set to non-nil value or it panics
				Header: Head,
			}, nil
		}
	})
	clientsInv = newClients(k8sclient, cInv)
	_ = os.MkdirAll("testcases/runanalysis/templates", os.ModePerm)
	_ = os.MkdirAll("testcases/runanalysis/provider", os.ModePerm)
	_ = os.MkdirAll("testcases/runanalysis/secrets", os.ModePerm)
	_, err = runAnalysis(clientsInv, resourceNames, "testcases/")
	assert.Equal(t, `analysis Error: Error in post processing canary Response: invalid character 'c' looking for beginning of object key string`, err.Error())

	_, err = runAnalysis(clients, resourceNames, "testcasesy/")
	assert.Equal(t, "provider config map validation error: open testcasesy/provider/providerConfig: no such file or directory\n Action Required: Provider config map has to be mounted on '/etc/config/provider' in AnalysisTemplate and must carry data element 'providerConfig'", err.Error())

	emptyFile, _ = os.Create("testcases/runanalysis/provider/providerConfig")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/analysis/providerConfig")
	_ = os.WriteFile("testcases/runanalysis/provider/providerConfig", input, 0644)
	_, err = runAnalysis(clients, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, "opsmx profile secret validation error: open testcases/runanalysis/secrets/user: no such file or directory\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'user'", err.Error())

	emptyFile, _ = os.Create("testcases/runanalysis/secrets/user")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/user")
	_ = os.WriteFile("testcases/runanalysis/secrets/user", input, 0644)
	emptyFile, _ = os.Create("testcases/runanalysis/secrets/opsmxIsdUrl")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/gate-url")
	_ = os.WriteFile("testcases/runanalysis/secrets/opsmxIsdUrl", input, 0644)
	emptyFile, _ = os.Create("testcases/runanalysis/secrets/cdIntegration")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/cd-Integration")
	_ = os.WriteFile("testcases/runanalysis/secrets/cdIntegration", input, 0644)
	emptyFile, _ = os.Create("testcases/runanalysis/secrets/sourceName")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/secret/source-name")
	_ = os.WriteFile("testcases/runanalysis/secrets/sourceName", input, 0644)

	emptyFile, _ = os.Create("testcases/runanalysis/provider/providerConfig")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/analysis/provideConfigGitops")
	_ = os.WriteFile("testcases/runanalysis/provider/providerConfig", input, 0644)
	_, err = runAnalysis(clients, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, "gitops 'loggytemp' template config map validation error: open testcases/runanalysis/templates/loggytemp: no such file or directory\n Action Required: Template has to be mounted on '/etc/config/templates' in AnalysisTemplate and must carry data element 'loggytemp'", err.Error())

	emptyFile, _ = os.Create("testcases/provider/providerConfig")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/analysis/basicCheckFail")
	_ = os.WriteFile("testcases/provider/providerConfig", input, 0644)
	_, err = runAnalysis(clients, resourceNames, "testcases/")
	assert.Equal(t, "provider config map validation error: intervalTime should be given along with lookBackType to perform interval analysis", err.Error())
	emptyFile, _ = os.Create("testcases/provider/providerConfig")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/analysis/failtimevariables")
	_ = os.WriteFile("testcases/provider/providerConfig", input, 0644)
	_, err = runAnalysis(clients, resourceNames, "testcases/")
	assert.Equal(t, "provider config map validation error: Error in parsing baselineStartTime: parsing time \"abc\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"abc\" as \"2006\"", err.Error())

	emptyFile, _ = os.Create("testcases/runanalysis/provider/providerConfig")
	emptyFile.Close()
	input, _ = os.ReadFile("testcases/analysis/providerConfig")
	_ = os.WriteFile("testcases/runanalysis/provider/providerConfig", input, 0644)
	cS := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(bytes.NewBufferString(`
			{
				"owner": "admin",
				"application": "testapp",
				"canaryResult": {
					"duration": "0 seconds",
					"lastUpdated": "2022-09-02 10:02:18.504",
					"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
					"overallScore": 100,
					"intervalNo": 1,
					"isLastRun": true,
					"overallResult": "HEALTHY",
					"message": "Canary Is HEALTHY",
					"errors": []
				},
				"launchedDate": "2022-09-02 10:02:18.504",
				"canaryConfig": {
					"combinedCanaryResultStrategy": "LOWEST",
					"minimumCanaryResultScore": 65.0,
					"name": "admin",
					"lifetimeMinutes": 30,
					"canaryAnalysisIntervalMins": 30,
					"maximumCanaryResultScore": 80.0
				},
				"id": "1424",
				"services": [],
				"status": {
					"complete": false,
					"status": "COMPLETED"
				}}
			`)),
			// Must be set to non-nil value or it panics
			Header: Head,
		}, nil
	})
	k8sclientS := jobFakeClient(cond)
	clientsS := newClients(k8sclientS, cS)
	_, err = runAnalysis(clientsS, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, nil, err)

	cCancel := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(bytes.NewBufferString(`
			{
				"owner": "admin",
				"application": "testapp",
				"canaryResult": {
					"duration": "0 seconds",
					"lastUpdated": "2022-09-02 10:02:18.504",
					"canaryReportURL": "https://opsmx.test.tst/ui/application/deploymentverification/testapp/1424",
					"overallScore": 100,
					"intervalNo": 1,
					"isLastRun": true,
					"overallResult": "HEALTHY",
					"message": "Canary Is HEALTHY",
					"errors": []
				},
				"launchedDate": "2022-09-02 10:02:18.504",
				"canaryConfig": {
					"combinedCanaryResultStrategy": "LOWEST",
					"minimumCanaryResultScore": 65.0,
					"name": "admin",
					"lifetimeMinutes": 30,
					"canaryAnalysisIntervalMins": 30,
					"maximumCanaryResultScore": 80.0
				},
				"id": "1424",
				"services": [],
				"status": {
					"complete": false,
					"status": "CANCELLED"
				}}
			`)),
			// Must be set to non-nil value or it panics
			Header: Head,
		}, nil
	})
	k8sclientCancel := jobFakeClient(cond)
	clientsCancel := newClients(k8sclientCancel, cCancel)
	_, err = runAnalysis(clientsCancel, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, nil, err)

	cHead := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(bytes.NewBufferString(`
			{
				"canaryId": 1424
			}
			`)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}, nil
	})
	clientsHead := newClients(k8sclientCancel, cHead)
	_, err = runAnalysis(clientsHead, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, "analysis Error: score url not found", err.Error())

	cError := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(bytes.NewBufferString(`
			{
				"canaryId": 1424,
				"message": "Error is Here",
				"error": "Here is Error",
				"errorMessage": "ErrorMessage"
			}
			`)),
			// Must be set to non-nil value or it panics
			Header: Head,
		}, nil
	})
	clientsError := newClients(k8sclientCancel, cError)
	_, err = runAnalysis(clientsError, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, "analysis Error: Here is Error\nMessage: Error is Here", err.Error())

	resourceNames = ResourceNames{
		podName: "pod",
		jobName: "job",
	}
	clientsPatchError := newClients(getFakeClient(map[string][]byte{}), c)
	_, err = runAnalysis(clientsPatchError, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, "jobs.batch \"job\" not found", err.Error())

	cUrlEroor := NewTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 404,
			Header:     make(http.Header),
		}, errors.New("Post \"https://opsmx.invalidurl.tst\": dial tcp: lookup https://opsmx.invalidurl.tst: no such host")
	})
	clientsUrlError := newClients(k8sclientS, cUrlEroor)
	_, err = runAnalysis(clientsUrlError, resourceNames, "testcases/runanalysis/")
	assert.Equal(t, "provider config map validation error: incorrect opsmxIsdUrl", err.Error())
	if _, err := os.Stat("testcases/secrets"); !os.IsNotExist(err) {
		os.RemoveAll("testcases/secrets")
	}
	if _, err := os.Stat("testcases/provider"); !os.IsNotExist(err) {
		os.RemoveAll("testcases/provider")
	}
	if _, err := os.Stat("testcases/runanalysis"); !os.IsNotExist(err) {
		os.RemoveAll("testcases/runanalysis")
	}
	_ = os.Remove("testcases/templates/invalid.txt")
	_ = os.Remove("testcases/templates/loggytemp")
	_ = os.Remove("testcases/templates/PrometheusMetricTemplate")
}

func TestRunner(t *testing.T) {
	httpclient := NewHttpClient()
	clients := newClients(getFakeClient(map[string][]byte{}), httpclient)
	err := runner(clients)
	assert.Equal(t, "analysisTemplate validation error: environment variable MY_POD_NAME is not set", err.Error())

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
	clients = newClients(k8sclient, httpclient)
	os.Setenv("MY_POD_NAME", "pod")
	err = runner(clients)
	assert.Equal(t, "pods \"pod\" not found", err.Error())
}
