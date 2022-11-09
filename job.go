package main

import (
	"context"
	"encoding/json"
	"os"

	"net/url"

	"fmt"
	"strings"

	"time"

	"errors"
)

const (
	templateApi                             = "/autopilot/api/v5/external/template?sha1=%s&templateType=%s&templateName=%s"
	v5configIdLookupURLFormat               = `/autopilot/api/v5/registerCanary`
	scoreUrlFormat                          = `/autopilot/v5/canaries/`
	resumeAfter                             = 3 * time.Second
	httpConnectionTimeout     time.Duration = 15 * time.Second
	defaultSecretName                       = "opsmx-profile"
	cdIntegrationArgoRollouts               = "argorollouts"
	cdIntegrationArgoCD                     = "argocd"
)

func runAnalysis(c *Clients, r ResourceNames) error {
	templateName := os.Getenv("PROVIDER_CONFIG")
	metric, err := getAnalysisTemplateData(templateName)
	if err != nil {
		return err
	}
	err = metric.basicChecks()
	if err != nil {
		return err
	}
	secretData, err := metric.getDataSecret()
	if err != nil {
		return err
	}
	canaryurl, err := url.JoinPath(secretData["gateUrl"], v5configIdLookupURLFormat)
	if err != nil {
		return err
	}
	//Get the epochs for Time variables and the lifetimeMinutes
	canaryStartTime, baselineStartTime, lifetimeMinutes, err := getTimeVariables(metric.BaselineStartTime, metric.CanaryStartTime, metric.EndTime, metric.LifetimeMinutes)
	if err != nil {
		return err
	}

	var intervalTime string
	if metric.IntervalTime != 0 {
		intervalTime = fmt.Sprintf("%d", metric.IntervalTime)
	} else {
		intervalTime = ""
	}

	var opsmxdelay string
	if metric.Delay != 0 {
		opsmxdelay = fmt.Sprintf("%d", metric.Delay)
	} else {
		opsmxdelay = ""
	}

	//Generate the payload
	payload := jobPayload{
		Application: metric.Application,
		SourceName:  secretData["sourceName"],
		SourceType:  secretData["cdIntegration"],
		CanaryConfig: canaryConfig{
			LifetimeMinutes: fmt.Sprintf("%d", lifetimeMinutes),
			LookBackType:    metric.LookBackType,
			IntervalTime:    intervalTime,
			Delays:          opsmxdelay,
			CanaryHealthCheckHandler: canaryHealthCheckHandler{
				MinimumCanaryResultScore: fmt.Sprintf("%d", metric.Marginal),
			},
			CanarySuccessCriteria: canarySuccessCriteria{
				CanaryResultScore: fmt.Sprintf("%d", metric.Pass),
			},
		},
		CanaryDeployments: []canaryDeployments{},
	}
	if metric.Services != nil || len(metric.Services) != 0 {
		deployment := canaryDeployments{
			BaselineStartTimeMs: baselineStartTime,
			CanaryStartTimeMs:   canaryStartTime,
			Baseline: &logMetric{
				Log:    map[string]map[string]string{},
				Metric: map[string]map[string]string{},
			},
			Canary: &logMetric{
				Log:    map[string]map[string]string{},
				Metric: map[string]map[string]string{},
			},
		}
		for i, item := range metric.Services {
			valid := false
			serviceName := fmt.Sprintf("service%d", i+1)
			if item.ServiceName != "" {
				serviceName = item.ServiceName
			}
			gateName := fmt.Sprintf("gate%d", i+1)
			if item.LogScopeVariables == "" && item.BaselineLogScope != "" || item.LogScopeVariables == "" && item.CanaryLogScope != "" {
				err := errors.New("missing log Scope placeholder for the provided baseline/canary")
				if err != nil {
					return err
				}
			}
			//For Log Analysis is to be added in analysis-run
			if item.LogScopeVariables != "" {
				//Check if no baseline or canary
				if item.BaselineLogScope != "" && item.CanaryLogScope == "" {
					err := errors.New("missing canary for log analysis")
					if err != nil {
						return err
					}
				}
				//Check if the number of placeholders provided dont match
				if len(strings.Split(item.LogScopeVariables, ",")) != len(strings.Split(item.BaselineLogScope, ",")) || len(strings.Split(item.LogScopeVariables, ",")) != len(strings.Split(item.CanaryLogScope, ",")) {
					err := errors.New("mismatch in number of log scope variables and baseline/canary log scope")
					if err != nil {
						return err
					}
				}
				if item.LogTemplateName == "" && metric.GlobalLogTemplate == "" {
					err := errors.New("provide either a service specific log template or global log template")
					if err != nil {
						return err
					}
				}

				baslineLogScope := getScopeValues(item.BaselineLogScope)
				//Add mandatory field for baseline
				deployment.Baseline.Log[serviceName] = map[string]string{
					item.LogScopeVariables: baslineLogScope,
					"serviceGate":          gateName,
				}

				canaryLogScope := getScopeValues(item.CanaryLogScope)
				//Add mandatory field for canary
				deployment.Canary.Log[serviceName] = map[string]string{
					item.LogScopeVariables: canaryLogScope,
					"serviceGate":          gateName,
				}

				var tempName string
				if item.LogTemplateName != "" {
					tempName = item.LogTemplateName
				} else {
					tempName = metric.GlobalLogTemplate
				}
				//Add service specific templateName
				deployment.Baseline.Log[serviceName]["template"] = tempName
				deployment.Canary.Log[serviceName]["template"] = tempName

				var templateData string
				if metric.GitOPS && item.LogTemplateVersion == "" {
					templateData, err = getTemplateData(c.client, secretData, tempName, "LOG")
					if err != nil {
						return err
					}
				}

				if metric.GitOPS && templateData != "" && item.LogTemplateVersion == "" {
					deployment.Baseline.Log[serviceName]["templateSha1"] = templateData
					deployment.Canary.Log[serviceName]["templateSha1"] = templateData
				}
				//Add non-mandatory field of Templateversion if provided
				if item.LogTemplateVersion != "" {
					deployment.Baseline.Log[serviceName]["templateVersion"] = item.LogTemplateVersion
					deployment.Canary.Log[serviceName]["templateVersion"] = item.LogTemplateVersion
				}
				valid = true
			}

			if item.MetricScopeVariables == "" && item.BaselineMetricScope != "" || item.MetricScopeVariables == "" && item.CanaryMetricScope != "" {
				err := errors.New("missing metric Scope placeholder for the provided baseline/canary")
				if err != nil {
					return err
				}
			}
			//For metric analysis is to be added in analysis-run
			if item.MetricScopeVariables != "" {
				//Check if no baseline or canary
				if item.BaselineMetricScope == "" || item.CanaryMetricScope == "" {
					err := errors.New("missing baseline/canary for metric analysis")
					if err != nil {
						return err
					}
				}
				//Check if the number of placeholders provided dont match
				if len(strings.Split(item.MetricScopeVariables, ",")) != len(strings.Split(item.BaselineMetricScope, ",")) || len(strings.Split(item.MetricScopeVariables, ",")) != len(strings.Split(item.CanaryMetricScope, ",")) {
					err := errors.New("mismatch in number of metric scope variables and baseline/canary metric scope")
					if err != nil {
						return err
					}
				}
				if item.MetricTemplateName == "" && metric.GlobalMetricTemplate == "" {
					err := errors.New("provide either a service specific metric template or global metric template")
					if err != nil {
						return err
					}
				}

				baselineMetricScope := getScopeValues(item.BaselineMetricScope)
				//Add mandatory field for baseline
				deployment.Baseline.Metric[serviceName] = map[string]string{
					item.MetricScopeVariables: baselineMetricScope,
					"serviceGate":             gateName,
				}

				canaryMetricScope := getScopeValues(item.CanaryMetricScope)
				//Add mandatory field for canary
				deployment.Canary.Metric[serviceName] = map[string]string{
					item.MetricScopeVariables: canaryMetricScope,
					"serviceGate":             gateName,
				}

				var tempName string
				if item.MetricTemplateName != "" {
					tempName = item.MetricTemplateName
				} else {
					tempName = metric.GlobalMetricTemplate
				}

				//Add templateName
				deployment.Baseline.Metric[serviceName]["template"] = tempName
				deployment.Canary.Metric[serviceName]["template"] = tempName

				var templateData string
				if metric.GitOPS && item.MetricTemplateVersion == "" {
					templateData, err = getTemplateData(c.client, secretData, tempName, "METRIC")
					if err != nil {
						return err
					}
				}

				if metric.GitOPS && templateData != "" && item.MetricTemplateVersion == "" {
					deployment.Baseline.Metric[serviceName]["templateSha1"] = templateData
					deployment.Canary.Metric[serviceName]["templateSha1"] = templateData
				}

				//Add non-mandatory field of Template Version if provided
				if item.MetricTemplateVersion != "" {
					deployment.Baseline.Metric[serviceName]["templateVersion"] = item.MetricTemplateVersion
					deployment.Canary.Metric[serviceName]["templateVersion"] = item.MetricTemplateVersion
				}
				valid = true

			}
			//Check if no logs or metrics were provided
			if !valid {
				err := errors.New("at least one of log or metric context must be included")
				if err != nil {
					return err
				}
			}
		}
		payload.CanaryDeployments = append(payload.CanaryDeployments, deployment)
	} else {
		//Check if no services were provided
		err = errors.New("no services provided")
		if err != nil {
			return err
		}
	}
	buffer, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	fmt.Println(string(buffer))
	data, err := makeRequest(c.client, "POST", canaryurl, string(buffer), secretData["user"])
	if err != nil {
		return err
	}
	//Struct to record canary Response
	type canaryResponse struct {
		Error    string      `json:"error,omitempty"`
		Message  string      `json:"message,omitempty"`
		CanaryId json.Number `json:"canaryId,omitempty"`
	}
	var canary canaryResponse

	json.Unmarshal(data, &canary)

	if canary.Error != "" {
		errMessage := fmt.Sprintf("Error: %s\nMessage: %s", canary.Error, canary.Message)
		err := errors.New(errMessage)
		if err != nil {
			return err
		}
	}
	scoreURL, _ := url.JoinPath(secretData["gateUrl"], scoreUrlFormat, canary.CanaryId.String())

	data, err = makeRequest(c.client, "GET", scoreURL, "", secretData["user"])
	if err != nil {
		return err
	}

	var status map[string]interface{}
	var reportUrlJson map[string]interface{}

	json.Unmarshal(data, &status)
	jsonBytes, _ := json.MarshalIndent(status["canaryResult"], "", "   ")
	json.Unmarshal(jsonBytes, &reportUrlJson)
	reportUrl := reportUrlJson["canaryReportURL"]

	ctx := context.TODO()
	// ar, err := c.argoclientset.ArgoprojV1alpha1().AnalysisRuns("ns").Get(ctx, r.analysisRunName, metav1.GetOptions{})
	// if err != nil {
	// 	return err
	// }

	cd := CanaryDetails{
		jobName:    r.jobName,
		canaryId:   canary.CanaryId.String(),
		gateUrl:    metric.GateUrl,
		reportUrl:  fmt.Sprintf("%s", reportUrl),
		phase:      "Running",
	}
	// patchCanaryDetails(c, ctx, r.analysisRunName, cd)
	patchJobCanaryDetails(c,ctx,r.jobName,cd)

	process := "RUNNING"
	//if the status is Running, pool again after delay
	for process == "RUNNING" {
		json.Unmarshal(data, &status)
		a, _ := json.MarshalIndent(status["status"], "", "   ")
		json.Unmarshal(a, &status)

		if status["status"] != "RUNNING" {
			process = "COMPLETED"
		} else {
			time.Sleep(resumeAfter)
			data, err = makeRequest(c.client, "GET", scoreURL, "", secretData["user"])
			if err != nil {
				return err
			}
		}
	}
	//if run is cancelled mid-run
	if status["status"] == "CANCELLED" {
		fmt.Printf("Analysis Cancelled")
	} else {
		//POST-Run process
		Phase, Score, err := metric.processResume(data)
		if err != nil {
			return err
		}
		if Phase == AnalysisPhaseSuccessful {

			fs := CanaryDetails{
				jobName:    r.jobName,
				canaryId:   canary.CanaryId.String(),
				gateUrl:    metric.GateUrl,
				reportUrl:  fmt.Sprintf("%s", reportUrl),
				phase:      "Running",
				value:      Score,
			}
			patchJobSuccessful(c,ctx,r.jobName,fs)
		}
		if Phase == AnalysisPhaseFailed {

			fs := CanaryDetails{
				jobName:    r.jobName,
				canaryId:   canary.CanaryId.String(),
				gateUrl:    metric.GateUrl,
				reportUrl:  fmt.Sprintf("%s", reportUrl),
				phase:      "Running",
				value:      Score,
			}
			patchJobFailedOthers(c,ctx,r.jobName,Phase,fs,2)
		}
		if Phase == AnalysisPhaseInconclusive {

			fs := CanaryDetails{
				jobName:    r.jobName,
				canaryId:   canary.CanaryId.String(),
				gateUrl:    metric.GateUrl,
				reportUrl:  fmt.Sprintf("%s", reportUrl),
				phase:      "Running",
				value:      Score,
			}
			patchJobFailedOthers(c,ctx,r.jobName,Phase,fs,3)
		}
	}
	return nil
}
