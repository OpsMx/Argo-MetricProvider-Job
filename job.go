package main

import (
	"context"
	"encoding/json"

	"net/url"

	"fmt"

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

func runAnalysis(c *Clients, r ResourceNames, analysispath string, userPath string, gateUrlPath string, sourceNamePath string, cdIntegrationPath string, templatePath string) error {
	metric, err := getAnalysisTemplateData(analysispath)
	if err != nil {
		return err
	}
	err = metric.basicChecks()
	if err != nil {
		return err
	}
	secretData, err := metric.getDataSecret(userPath, gateUrlPath, sourceNamePath, cdIntegrationPath)
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

	payload, err := metric.getPayload(c, secretData, canaryStartTime, baselineStartTime, lifetimeMinutes, templatePath)
	if err != nil {
		return err
	}

	data, scoreURL, err := makeRequest(c.client, "POST", canaryurl, payload, secretData["user"])
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

	data, _, err = makeRequest(c.client, "GET", scoreURL, "", secretData["user"])
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

	cd := CanaryDetails{
		jobName:   r.jobName,
		canaryId:  canary.CanaryId.String(),
		reportUrl: fmt.Sprintf("%s", reportUrl),
	}

	err = patchJobCanaryDetails(c.kubeclientset, ctx, cd)
	if err != nil {
		return err
	}

	retryScorePool := 5
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
			data, _, err = makeRequest(c.client, "GET", scoreURL, "", secretData["user"])
			if err != nil && retryScorePool == 0 {
				return err
			} else {
				retryScorePool -= 1
			}
		}
	}
	//if run is cancelled mid-run
	if status["status"] == "CANCELLED" {
		err = patchJobCancelled(c.kubeclientset, ctx, r.jobName)
		if err != nil {
			return err
		}
		logErrorAndExit(4, nil)
	} else {
		//POST-Run process
		Phase, Score, err := metric.processResume(data)
		if err != nil {
			return err
		}
		if Phase == AnalysisPhaseSuccessful {

			fs := CanaryDetails{
				jobName:   r.jobName,
				canaryId:  canary.CanaryId.String(),
				reportUrl: fmt.Sprintf("%s", reportUrl),
				value:     Score,
			}
			err = patchJobSuccessful(c.kubeclientset, ctx, fs)
			if err != nil {
				return err
			}
		}
		if Phase == AnalysisPhaseFailed {

			fs := CanaryDetails{
				jobName:   r.jobName,
				canaryId:  canary.CanaryId.String(),
				reportUrl: fmt.Sprintf("%s", reportUrl),
				value:     Score,
			}
			err = patchJobFailedInconclusive(c.kubeclientset, ctx, Phase, fs)
			if err != nil {
				return err
			}
			logErrorAndExit(2, nil)
		}
		if Phase == AnalysisPhaseInconclusive {

			fs := CanaryDetails{
				jobName:   r.jobName,
				canaryId:  canary.CanaryId.String(),
				reportUrl: fmt.Sprintf("%s", reportUrl),
				value:     Score,
			}
			err = patchJobFailedInconclusive(c.kubeclientset, ctx, Phase, fs)
			if err != nil {
				return err
			}
			logErrorAndExit(3, nil)
		}
	}
	return nil
}
