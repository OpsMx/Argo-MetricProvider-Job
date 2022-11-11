package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuncGetAnalysisTemplateData(t *testing.T) {
	metric, err := getAnalysisTemplateData("/home/user/Argo-MetricProvider-Job/etc/config/provider/providerConfig")
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
	_, err = getAnalysisTemplateData("/home/user/Argo-MetricProvider-Job/invalidyamltest/invalid")
	assert.Equal(t, err.Error(), "yaml: line 9: mapping values are not allowed in this context")
}

var negativeTests = []struct {
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

func TestGenericNegativeTestsRun(t *testing.T) {
	for i, test := range negativeTests {
		fmt.Printf("\n%d\n", i)
		err := test.metric.basicChecks()
		assert.Equal(t, err.Error(), test.message)
	}
}
