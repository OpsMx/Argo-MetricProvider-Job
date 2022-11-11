package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuncGetAnalysisTemplateData(t *testing.T) {
	metric, err := getAnalysisTemplateData("/home/user/Argo-MetricProvider-Job/analysis/providerConfig")
	checkMetric := OPSMXMetric{
		Application:     "final-job",
		User:            "admin",
		GateUrl:         "https://isd.opsmx.net/",
		LifetimeMinutes: 3,
		IntervalTime:    3,
		LookBackType:    "sliding",
		Pass:            80,
		Marginal:        60,
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
	_, err = getAnalysisTemplateData("/etc/config/provider/providerConfig")
	assert.Equal(t, err.Error(), "open /etc/config/provider/providerConfig: no such file or directory")
	_, err = getAnalysisTemplateData("/home/user/Argo-MetricProvider-Job/analysis/invalid")
	assert.Equal(t, err.Error(), "yaml: line 9: mapping values are not allowed in this context")
}

var basicChecks = []struct {
	metric  OPSMXMetric
	message string
}{
	//Test case for no lifetimeMinutes, Baseline/Canary start time
	{
		metric: OPSMXMetric{
			GateUrl:     "https://opsmx.test.tst",
			Application: "testapp",
			User:        "admin",
			Pass:        80,
			Marginal:    65,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "either provide lifetimeMinutes or end time",
	},
	//Test case for Pass score less than marginal
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   30,

			Pass:     60,
			Marginal: 80,

			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "pass score cannot be less than marginal score",
	},
	//Test case for no lifetimeMinutes & EndTime
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "either provide lifetimeMinutes or end time",
	},
	//Test case when end time given and baseline and canary start time not same
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-08-02T12:45:00Z",
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "both start time should be kept same in case of using end time argument",
	},
	//Test case when lifetimeMinutes is less than 3 minutes
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   2,
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "lifetime minutes cannot be less than 3 minutes",
	},
	//Test case when intervalTime is less than 3 minutes
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      2,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "interval time cannot be less than 3 minutes",
	},
	//Test case when intervalTime is given but lookBackType is not given
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "prom",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascienece-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemp",
				},
			},
		},
		message: "interval time is given and lookbacktype is required to run interval analysis",
	},

	//Test case when intervalTime is not given but lookBackType is given
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "prom",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascienece-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logtemp",
				},
			},
		},
		message: "lookbacktype is given and interval time is required to run interval analysis",
	}, /*
		//Test case when improper URL
		{
			metric: OPSMXMetric{
				GateUrl:           "	",
				Application:       "testapp",
				BaselineStartTime: "2022-08-02T14:15:00Z",
				CanaryStartTime:   "2022-08-02T13:15:00Z",
				LifetimeMinutes:   60,
				IntervalTime:      3,
				LookBackType:      "growing",
				Pass:              80,
				Marginal:          60,
				Services: []OPSMXService{
					{
						MetricScopeVariables: "job_name",
						BaselineMetricScope:  "oes-sapor-br",
						CanaryMetricScope:    "oes-sapor-cr",
						MetricTemplateName:   "prom",
						LogScopeVariables:    "kubernetes.container_name",
						BaselineLogScope:     "oes-datascienece-br",
						CanaryLogScope:       "oes-datascience-cr",
						LogTemplateName:      "logtemp",
					},
				},
			},
			message: "parse \"\\t\": net/url: invalid control character in URL",
		},*/
}

func TestBasicChecks(t *testing.T) {
	for _, test := range basicChecks {
		err := test.metric.basicChecks()
		assert.Equal(t, err.Error(), test.message)
	}
	metric := OPSMXMetric{
		GateUrl:           "https://opsmx.test.tst",
		Application:       "testapp",
		User:              "admin",
		BaselineStartTime: "2022-08-02T13:15:00Z",
		CanaryStartTime:   "2022-08-02T13:15:00Z",
		LifetimeMinutes:   30,
		Pass:              100,
		Marginal:          80,

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
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-O8-02T13:15:00Z",
			LifetimeMinutes:   30,
			Pass:              100,
			Marginal:          80,

			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "parsing time \"2022-O8-02T13:15:00Z\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"O8-02T13:15:00Z\" as \"01\"",
	},
	//Test case for inappropriate time format baseline
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-O8-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   30,
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "parsing time \"2022-O8-02T13:15:00Z\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"O8-02T13:15:00Z\" as \"01\"",
	},
	//Test case for inappropriate time format endTime
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			User:              "admin",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-O8-02T13:15:00Z",
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "parsing time \"2022-O8-02T13:15:00Z\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"O8-02T13:15:00Z\" as \"01\"",
	},
	//Test case for when end time is less than start time
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-08-02T12:45:00Z",
			Pass:              80,
			Marginal:          60,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-datascience-br",
					CanaryMetricScope:    "oes-datascience-cr",
					MetricTemplateName:   "metrictemplate",
				},
			},
		},
		message: "start time cannot be greater than end time",
	},
}

func TestGetTimeVariables(t *testing.T) {
	for _, test := range checkTimeVariables {
		_, _, _, err := getTimeVariables(test.metric.BaselineStartTime, test.metric.CanaryStartTime, test.metric.EndTime, test.metric.LifetimeMinutes)
		assert.Equal(t, err.Error(), test.message)
	}
	metric := OPSMXMetric{
		GateUrl:         "https://opsmx.test.tst",
		Application:     "testapp",
		User:            "admin",
		LifetimeMinutes: 30,
		Pass:            80,
		Marginal:        60,
		Services: []OPSMXService{
			{
				MetricScopeVariables: "job_name",
				BaselineMetricScope:  "oes-datascience-br",
				CanaryMetricScope:    "oes-datascience-cr",
				MetricTemplateName:   "metrictemplate",
			},
		},
	}
	_, _, _, err := getTimeVariables(metric.BaselineStartTime, metric.CanaryStartTime, metric.EndTime, metric.LifetimeMinutes)
	assert.Equal(t, err, nil)

	metric = OPSMXMetric{
		GateUrl:           "https://opsmx.test.tst",
		Application:       "testapp",
		BaselineStartTime: "2022-08-02T13:15:00Z",
		CanaryStartTime:   "2022-08-02T13:15:00Z",
		EndTime:           "2022-08-02T13:45:00Z",
		Pass:              80,
		Marginal:          60,
		Services: []OPSMXService{
			{
				MetricScopeVariables: "job_name",
				BaselineMetricScope:  "oes-datascience-br",
				CanaryMetricScope:    "oes-datascience-cr",
				MetricTemplateName:   "metrictemplate",
			},
		},
	}
	_, _, lifetimeMinutes, err := getTimeVariables(metric.BaselineStartTime, metric.CanaryStartTime, metric.EndTime, metric.LifetimeMinutes)
	assert.Equal(t, err, nil)
	assert.Equal(t, lifetimeMinutes, 30)
}

func TestSecret(t *testing.T) {
	metric := OPSMXMetric{
		Application:     "final-job",
		LifetimeMinutes: 3,
		IntervalTime:    3,
		LookBackType:    "sliding",
		Pass:            80,
		Marginal:        60,
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

	_, err := metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/users", "/home/user/Argo-MetricProvider-Job/secret/gate-url", "/home/user/Argo-MetricProvider-Job/secret/source-name", "/home/user/Argo-MetricProvider-Job/secret/cd-Integration")
	assert.Equal(t, err.Error(), "open /home/user/Argo-MetricProvider-Job/secret/users: no such file or directory")

	_, err = metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/user", "/home/user/Argo-MetricProvider-Job/secret/gate-urls", "/home/user/Argo-MetricProvider-Job/secret/source-name", "/home/user/Argo-MetricProvider-Job/secret/cd-Integration")
	assert.Equal(t, err.Error(), "open /home/user/Argo-MetricProvider-Job/secret/gate-urls: no such file or directory")

	_, err = metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/user", "/home/user/Argo-MetricProvider-Job/secret/gate-url", "/home/user/Argo-MetricProvider-Job/secret/source-names", "/home/user/Argo-MetricProvider-Job/secret/cd-Integration")
	assert.Equal(t, err.Error(), "open /home/user/Argo-MetricProvider-Job/secret/source-names: no such file or directory")

	_, err = metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/user", "/home/user/Argo-MetricProvider-Job/secret/gate-url", "/home/user/Argo-MetricProvider-Job/secret/source-name", "/home/user/Argo-MetricProvider-Job/secret/cd-Integrations")
	assert.Equal(t, err.Error(), "open /home/user/Argo-MetricProvider-Job/secret/cd-Integrations: no such file or directory")

	secretData, err := metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/user", "/home/user/Argo-MetricProvider-Job/secret/gate-url", "/home/user/Argo-MetricProvider-Job/secret/source-name", "/home/user/Argo-MetricProvider-Job/secret/cd-Integration")
	assert.Equal(t, err, nil)
	checkSecretData := map[string]string{
		"cdIntegration": "argocd",
		"sourceName":    "argocd06",
		"gateUrl":       "www.opsmx.com",
		"user":          "admins",
	}
	assert.Equal(t, checkSecretData, secretData)

	secretData, err = metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/user", "/home/user/Argo-MetricProvider-Job/secret/gate-url", "/home/user/Argo-MetricProvider-Job/secret/source-name", "/home/user/Argo-MetricProvider-Job/secret/cd-Integration-False")
	assert.Equal(t, err, nil)
	checkSecretData = map[string]string{
		"cdIntegration": "argorollouts",
		"sourceName":    "argocd06",
		"gateUrl":       "www.opsmx.com",
		"user":          "admins",
	}
	assert.Equal(t, checkSecretData, secretData)

	_, err = metric.getDataSecret("/home/user/Argo-MetricProvider-Job/secret/user", "/home/user/Argo-MetricProvider-Job/secret/gate-url", "/home/user/Argo-MetricProvider-Job/secret/source-name", "/home/user/Argo-MetricProvider-Job/secret/cd-Integration-Invalid")
	assert.Equal(t, err.Error(), "cd-integration should be either true or false")
}

var successfulPayload = []struct {
	metric                OPSMXMetric
	payloadRegisterCanary string
}{
	//Test case for basic function of Single Service feature
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			LifetimeMinutes:   30,
			IntervalTime:      3,
			Delay:             1,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          65,
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
											"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
											"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
											"minimumCanaryResultScore": "65"
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
			GateUrl:              "https://opsmx.test.tst",
			Application:          "multiservice",
			BaselineStartTime:    "2022-08-10T13:15:00Z",
			CanaryStartTime:      "2022-08-10T13:15:00Z",
			EndTime:              "2022-08-10T13:45:10Z",
			GlobalMetricTemplate: "metricTemplate",
			Pass:                 80,
			Marginal:             65,
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
						"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
						"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
						"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
						"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			GlobalLogTemplate: "logTemplate",
			Pass:              80,
			Marginal:          65,
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
						"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
						"minimumCanaryResultScore": "65"
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
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
		},
		message: "no services provided",
	},
	//Test case for No log & Metric analysis
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
			Services: []OPSMXService{
				{
					ServiceName: "service1",
				},
				{
					ServiceName: "service2",
				},
			},
		},
		message: "at least one of log or metric context must be included",
	},
	//Test case for mismatch in log scope variables and baseline/canary log scope
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
		message: "mismatch in number of log scope variables and baseline/canary log scope",
	},

	//Test case for mismatch in metric scope variables and baseline/canary metric scope
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			User:              "admin",
			Application:       "multiservice",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:10Z",
			Pass:              80,
			Marginal:          65,
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
		message: "mismatch in number of metric scope variables and baseline/canary metric scope",
	},
	//Test case when baseline or canary logplaceholder is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing canary for log analysis",
	},

	//Test case when baseline or canary metricplaceholder is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing baseline/canary for metric analysis",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "provide either a service specific log template or global log template",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "provide either a service specific metric template or global metric template",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing log Scope placeholder for the provided baseline/canary",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing log Scope placeholder for the provided baseline/canary",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing log Scope placeholder for the provided baseline/canary",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing metric Scope placeholder for the provided baseline/canary",
	},

	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing metric Scope placeholder for the provided baseline/canary",
	},

	//Test case when global and service specific template is missing
	{
		metric: OPSMXMetric{
			GateUrl:           "https://opsmx.test.tst",
			Application:       "testapp",
			BaselineStartTime: "2022-08-02T14:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   60,
			IntervalTime:      3,
			LookBackType:      "growing",
			Pass:              80,
			Marginal:          60,
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
		message: "missing metric Scope placeholder for the provided baseline/canary",
	},
}

func TestPayload(t *testing.T) {
	httpclient := NewHttpClient()
	clients := newClients(nil, httpclient)
	SecretData := map[string]string{
		"cdIntegration": "argocd",
		"sourceName":    "sourcename",
		"gateUrl":       "www.opsmx.com",
		"user":          "admins",
	}

	for _, test := range successfulPayload {
		canaryStartTime, baselineStartTime, lifetimeMinutes, err := getTimeVariables(test.metric.BaselineStartTime, test.metric.CanaryStartTime, test.metric.EndTime, test.metric.LifetimeMinutes)
		assert.Equal(t, nil, err)
		payload, err := test.metric.getPayload(clients, SecretData, canaryStartTime, baselineStartTime, lifetimeMinutes, "notrequired")
		assert.Equal(t, nil, err)
		processedPayload := strings.Replace(strings.Replace(strings.Replace(test.payloadRegisterCanary, "\n", "", -1), "\t", "", -1), " ", "", -1)
		assert.Equal(t, processedPayload, payload)
	}
	for _, test := range failPayload {
		canaryStartTime, baselineStartTime, lifetimeMinutes, err := getTimeVariables(test.metric.BaselineStartTime, test.metric.CanaryStartTime, test.metric.EndTime, test.metric.LifetimeMinutes)
		assert.Equal(t, nil, err)
		_, err = test.metric.getPayload(clients, SecretData, canaryStartTime, baselineStartTime, lifetimeMinutes, "notrequired")
		assert.Equal(t, test.message, err.Error())
	}
}

func TestGitops(t *testing.T) {
	httpclient := NewHttpClient()
	clients := newClients(nil, httpclient)
	SecretData := map[string]string{
		"cdIntegration": "argocd",
		"sourceName":    "sourcename",
		"gateUrl":       "http://www.opsmx.com",
		"user":          "admins",
	}
	metric := OPSMXMetric{
		GateUrl:           "https://opsmx.test.tst",
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
		Marginal:          65,
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
								"minimumCanaryResultScore": "65"
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
						"kubernetes.pod_name":".*.*",
						"serviceGate":"gate1",
						"template":"loggytemp",
						"templateSha1":"1fd53480333cb618aa05ce901a051263efabe3cd"}
					  }},
					"baseline": {
						"log": {"service1":{
						"kubernetes.pod_name":".*.*",
						"serviceGate":"gate1",
						"template":"loggytemp",
						"templateSha1":"1fd53480333cb618aa05ce901a051263efabe3cd"}}
					  }
					}
		  ]
	}`

	metric.Services = append(metric.Services, services)
	canaryStartTime, baselineStartTime, lifetimeMinutes, err := getTimeVariables(metric.BaselineStartTime, metric.CanaryStartTime, metric.EndTime, metric.LifetimeMinutes)
	assert.Equal(t, nil, err)
	_, err = metric.getPayload(clients, SecretData, canaryStartTime, baselineStartTime, lifetimeMinutes, "incorrect/%s")
	assert.Equal(t, "open incorrect/loggytemp: no such file or directory", err.Error())
	payload, err := metric.getPayload(clients, SecretData, canaryStartTime, baselineStartTime, lifetimeMinutes, "gitops/%s")
	assert.Equal(t, nil, err)
	processedPayload := strings.Replace(strings.Replace(strings.Replace(checkPayload, "\n", "", -1), "\t", "", -1), " ", "", -1)
	assert.Equal(t, processedPayload, payload)

	metric = OPSMXMetric{
		GateUrl:           "https://opsmx.test.tst",
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
		Marginal:          65,
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
								"minimumCanaryResultScore": "65"
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
						"kubernetes.pod_name":".*.*",
						"serviceGate":"gate1",
						"template":"PrometheusMetricTemplate",
						"templateSha1":"445b4c60855cd618b070e91ee232860e40e23d9c"}
					  }},
					"baseline": {
						"metric": {"service1":{
						"kubernetes.pod_name":".*.*",
						"serviceGate":"gate1",
						"template":"PrometheusMetricTemplate",
						"templateSha1":"445b4c60855cd618b070e91ee232860e40e23d9c"}}
					  }
					}
		  ]
	}`

	metric.Services = append(metric.Services, services)
	canaryStartTime, baselineStartTime, lifetimeMinutes, err = getTimeVariables(metric.BaselineStartTime, metric.CanaryStartTime, metric.EndTime, metric.LifetimeMinutes)
	assert.Equal(t, nil, err)
	payload, err = metric.getPayload(clients, SecretData, canaryStartTime, baselineStartTime, lifetimeMinutes, "gitops/%s")
	assert.Equal(t, nil, err)
	processedPayload = strings.Replace(strings.Replace(strings.Replace(checkPayload, "\n", "", -1), "\t", "", -1), " ", "", -1)
	assert.Equal(t, processedPayload, payload)

	invalidjsonmetric := OPSMXMetric{
		Application:     "final-job",
		LifetimeMinutes: 3,
		IntervalTime:    3,
		LookBackType:    "sliding",
		Pass:            80,
		Marginal:        60,
		GitOPS:          true,
		Services:        []OPSMXService{},
	}
	invalidjsonservices := OPSMXService{
		LogTemplateName:      "loggytemp.txt",
		LogScopeVariables:    "kubernetes.pod_name",
		BaselineLogScope:     ".*{{env.STABLE_POD_HASH}}.*",
		CanaryLogScope:       ".*{{env.LATEST_POD_HASH}}.*",
		MetricTemplateName:   "PrometheusMetricTemplate",
		MetricScopeVariables: "${namespace_key},${pod_key},${app_name}",
		BaselineMetricScope:  "argocd,{{env.STABLE_POD_HASH}},demoapp-issuegen",
		CanaryMetricScope:    "argocd,{{env.LATEST_POD_HASH}},demoapp-issuegen",
	}
	invalidjsonmetric.Services = append(invalidjsonmetric.Services, invalidjsonservices)
	_, err = invalidjsonmetric.getPayload(clients, SecretData, canaryStartTime, baselineStartTime, lifetimeMinutes, "gitops/invalid/%s")
	assert.Equal(t, "invalid template json provided", err.Error())
}
