package main

import (
	"context"
	"encoding/json"

	"net/url"

	"fmt"

	"time"

	"errors"

	log "github.com/sirupsen/logrus"
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

func runAnalysis(c *Clients, r ResourceNames, basePath string) (ExitCode, error) {
	log.Info("starting the getAnalysisTemplateData function")
	metric, err := getAnalysisTemplateData(basePath)
	if err != nil {
		return ReturnCodeError, err
	}
	log.Info("provider config data retrieved successfully")
	log.Info("performing basic checks")
	err = metric.basicChecks()
	if err != nil {
		return ReturnCodeError, err
	}
	log.Info("basic checks completed successfully")
	log.Info("getting the data from the secret")
	secretData, err := metric.getDataSecret(basePath)
	if err != nil {
		return ReturnCodeError, err
	}
	log.Info("secret data retrieved successfully")
	if err := metric.checkGateUrl(c, secretData["gateUrl"]); err != nil {
		return ReturnCodeError, err
	}
	canaryurl, err := url.JoinPath(secretData["gateUrl"], v5configIdLookupURLFormat)
	if err != nil {
		return ReturnCodeError, err
	}
	//Get the epochs for Time variables and the lifetimeMinutes
	err = metric.getTimeVariables()
	if err != nil {
		return ReturnCodeError, err
	}
	log.Info("generating the payload")
	payload, err := metric.generatePayload(c, secretData, basePath)
	if err != nil {
		return ReturnCodeError, err
	}
	log.Info(payload)
	log.Info("sending a POST request to registerCanary with the payload")
	data, scoreURL, err := makeRequest(c.client, "POST", canaryurl, payload, secretData["user"])
	if err != nil {
		return ReturnCodeError, err
	}
	//Struct to record canary Response
	type canaryResponse struct {
		Error    string      `json:"error,omitempty"`
		Message  string      `json:"message,omitempty"`
		CanaryId json.Number `json:"canaryId,omitempty"`
	}
	var canary canaryResponse

	err = json.Unmarshal(data, &canary)
	if err != nil {
		return ReturnCodeError, err
	}
	log.Info("register canary response ", canary)
	if canary.Error != "" {
		errMessage := fmt.Sprintf("Error: %s\nMessage: %s", canary.Error, canary.Message)
		err := errors.New(errMessage)
		if err != nil {
			return ReturnCodeError, err
		}
	}
	if scoreURL == "" {
		return ReturnCodeError, errors.New("score url not found")
	}
	data, _, err = makeRequest(c.client, "GET", scoreURL, "", secretData["user"])
	if err != nil {
		return ReturnCodeError, err
	}

	var status map[string]interface{}
	var reportUrlJson map[string]interface{}

	err = json.Unmarshal(data, &status)
	if err != nil {
		errorMessage := fmt.Sprintf("Error in post processing canary Response. Error: %v", err)
		return ReturnCodeError, errors.New(errorMessage)
	}
	jsonBytes, _ := json.MarshalIndent(status["canaryResult"], "", "   ")
	err = json.Unmarshal(jsonBytes, &reportUrlJson)
	if err != nil {
		return ReturnCodeError, err
	}
	reportUrl := reportUrlJson["canaryReportURL"]

	ctx := context.TODO()

	cd := CanaryDetails{
		user:      secretData["user"],
		jobName:   r.jobName,
		canaryId:  canary.CanaryId.String(),
		reportUrl: fmt.Sprintf("%s", reportUrl),
	}
	log.Info("starting the patching operation of the canary details to the Job")
	err = patchJobCanaryDetails(ctx, c.kubeclientset, cd)
	if err != nil {
		return ReturnCodeError, err
	}

	retryScorePool := 5
	process := "RUNNING"
	//if the status is Running, pool again after delay
	for process == "RUNNING" {
		err = json.Unmarshal(data, &status)
		if err != nil {
			errorMessage := fmt.Sprintf("Error in post processing canary Response. Error: %v", err)
			return ReturnCodeError, errors.New(errorMessage)
		}
		a, _ := json.MarshalIndent(status["status"], "", "   ")
		err = json.Unmarshal(a, &status)
		if err != nil {
			return ReturnCodeError, err
		}

		if status["status"] != "RUNNING" {
			process = "COMPLETED"
		} else {
			time.Sleep(resumeAfter)
			data, _, err = makeRequest(c.client, "GET", scoreURL, "", secretData["user"])
			if err != nil && retryScorePool == 0 {
				errorMessage := fmt.Sprintf("Error in getting canary Response. Error: %v", err)
				return ReturnCodeError, errors.New(errorMessage)
			} else {
				retryScorePool -= 1
			}
		}
	}
	//if run is cancelled mid-run
	if status["status"] == "CANCELLED" {
		log.Info("starting the patching operation for a CANCELLED operation")
		err = patchJobCancelled(ctx, c.kubeclientset, r.jobName)
		if err != nil {
			return ReturnCodeError, err
		}
		return ReturnCodeCancelled, nil
	}
	log.Info("final response ", string(data))
	//POST-Run process
	Phase, Score, err := metric.processResume(data)
	if err != nil {
		return ReturnCodeError, err
	}
	if Phase == AnalysisPhaseSuccessful {
		fs := CanaryDetails{
			user:      secretData["user"],
			jobName:   r.jobName,
			canaryId:  canary.CanaryId.String(),
			reportUrl: fmt.Sprintf("%s", reportUrl),
			value:     Score,
		}
		log.Infof("starting the patching operation for a %s operation", AnalysisPhaseSuccessful)
		err = patchJobSuccessful(ctx, c.kubeclientset, fs)
		if err != nil {
			return ReturnCodeError, err
		}
	}
	if Phase == AnalysisPhaseFailed {
		fs := CanaryDetails{
			user:      secretData["user"],
			jobName:   r.jobName,
			canaryId:  canary.CanaryId.String(),
			reportUrl: fmt.Sprintf("%s", reportUrl),
			value:     Score,
		}
		log.Infof("starting the patching operation for a %s operation", AnalysisPhaseFailed)
		err = patchJobFailedInconclusive(ctx, c.kubeclientset, Phase, fs)
		if err != nil {
			return ReturnCodeError, err
		}
		return ReturnCodeFailed, nil
	}
	return ReturnCodeSuccess, nil
}
