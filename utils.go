package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func logErrorExit1(err error) {
	if err != nil {
		log.Infof("Inside the exit 1 block")
		log.Error(err)
		os.Exit(1)
	}
}

func getAnalysisRunNameFromPod(p *Clients, ctx context.Context, podName string) (string, error) {
	//TODO - Introduce more checks, remove prints
	ns := defaults.Namespace()
	jobName := getJobNameFromPod(podName)
	log.Infof("The job name is %s", jobName)

	job, err := p.kubeclientset.BatchV1().Jobs(ns).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	parent := job.OwnerReferences[0]
	var analysisRunName string
	if parent.Kind == "AnalysisRun" {
		analysisRunName = parent.Name
	}
	return analysisRunName, nil

}

func getJobNameFromPod(podName string) string {
	// TODO- Retrieve data from the last hyphen and use error if required
	return podName[:len(podName)-6]

}

func isJSON(s string) bool {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(s), &j); err != nil {
		return false
	}
	return true
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func makeRequest(client http.Client, requestType string, url string, body string, user string) ([]byte, error) {
	reqBody := strings.NewReader(body)
	req, _ := http.NewRequest(
		requestType,
		url,
		reqBody,
	)

	req.Header.Set("x-spinnaker-user", user)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}
	return data, err
}

// Check few conditions pre-analysis
func (metric *OPSMXMetric) basicChecks() error {
	if metric.Threshold.Pass <= metric.Threshold.Marginal {
		return errors.New("pass score cannot be less than marginal score")
	}
	if metric.LifetimeMinutes == 0 && metric.EndTime == "" {
		return errors.New("either provide lifetimeMinutes or end time")
	}
	if metric.CanaryStartTime != metric.BaselineStartTime && metric.LifetimeMinutes == 0 {
		return errors.New("both start time should be kept same in case of using end time argument")
	}
	if metric.LifetimeMinutes != 0 && metric.LifetimeMinutes < 3 {
		return errors.New("lifetime minutes cannot be less than 3 minutes")
	}
	if metric.IntervalTime != 0 && metric.IntervalTime < 3 {
		return errors.New("interval time cannot be less than 3 minutes")
	}
	if metric.LookBackType != "" && metric.IntervalTime == 0 {
		return errors.New("lookbacktype is given and interval time is required to run interval analysis")
	}
	if metric.LookBackType == "" && metric.IntervalTime != 0 {
		return errors.New("interval time is given and lookbacktype is required to run interval analysis")
	}
	return nil
}

// Return epoch values of the specific time provided along with lifetimeMinutes for the Run
func getTimeVariables(baselineTime string, canaryTime string, endTime string, lifetimeMinutes int) (string, string, int, error) {

	var canaryStartTime string
	var baselineStartTime string
	tm := time.Now()

	if canaryTime == "" {
		canaryStartTime = fmt.Sprintf("%d", tm.UnixNano()/int64(time.Millisecond))
	} else {
		tsStart, err := time.Parse(time.RFC3339, canaryTime)
		if err != nil {
			return "", "", 0, err
		}
		canaryStartTime = fmt.Sprintf("%d", tsStart.UnixNano()/int64(time.Millisecond))
	}

	if baselineTime == "" {
		baselineStartTime = fmt.Sprintf("%d", tm.UnixNano()/int64(time.Millisecond))
	} else {
		tsStart, err := time.Parse(time.RFC3339, baselineTime)
		if err != nil {
			return "", "", 0, err
		}
		baselineStartTime = fmt.Sprintf("%d", tsStart.UnixNano()/int64(time.Millisecond))
	}

	//If lifetimeMinutes not given calculate using endTime
	if lifetimeMinutes == 0 {
		tsEnd, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			return "", "", 0, err
		}
		if canaryTime != "" && canaryTime > endTime {
			err := errors.New("start time cannot be greater than end time")
			return "", "", 0, err
		}
		tsStart := tm
		if canaryTime != "" {
			tsStart, _ = time.Parse(time.RFC3339, canaryTime)
		}
		tsDifference := tsEnd.Sub(tsStart)
		min, _ := time.ParseDuration(tsDifference.String())
		lifetimeMinutes = int(roundFloat(min.Minutes(), 0))
	}
	return canaryStartTime, baselineStartTime, lifetimeMinutes, nil
}

func getAnalysisTemplateData(template string, Namespace string, kubeclientset kubernetes.Interface) (OPSMXMetric, error) {
	analysisTemplateData, err := kubeclientset.CoreV1().ConfigMaps(Namespace).Get(context.TODO(), template, metav1.GetOptions{})
	if err != nil {
		return OPSMXMetric{}, err
	}

	if analysisTemplateData == nil || analysisTemplateData.Data["Template"] != "ISDAnalysisTemplate" {
		err = errors.New("analysis template not found")
		return OPSMXMetric{}, err
	}

	var lifetimeMinutes int
	if analysisTemplateData.Data["lifetimeMinutes"] != "" {
		lifetimeMinutes, err = strconv.Atoi(analysisTemplateData.Data["lifetimeMinutes"])
		if err != nil {
			return OPSMXMetric{}, err
		}
	}

	var intervalTime int
	if analysisTemplateData.Data["intervalTime"] != "" {
		intervalTime, err = strconv.Atoi(analysisTemplateData.Data["intervalTime"])
		if err != nil {
			return OPSMXMetric{}, err
		}
	}

	var delay int
	if analysisTemplateData.Data["delay"] != "" {
		delay, err = strconv.Atoi(analysisTemplateData.Data["delay"])
		if err != nil {
			return OPSMXMetric{}, err
		}
	}

	var gitops bool
	if analysisTemplateData.Data["gitops"] != "" {
		gitops, err = strconv.ParseBool(analysisTemplateData.Data["gitops"])
		if err != nil {
			return OPSMXMetric{}, err
		}
	}

	var pass int
	if analysisTemplateData.Data["pass"] != "" {
		pass, err = strconv.Atoi(analysisTemplateData.Data["pass"])
		if err != nil {
			return OPSMXMetric{}, err
		}
	}

	var marginal int
	if analysisTemplateData.Data["marginal"] != "" {
		marginal, err = strconv.Atoi(analysisTemplateData.Data["marginal"])
		if err != nil {
			return OPSMXMetric{}, err
		}
	}

	var services OPSMXMetric
	if analysisTemplateData.Data["services"] != "" {
		if err := yaml.Unmarshal([]byte(analysisTemplateData.Data["services"]), &services); err != nil {
			return OPSMXMetric{}, err
		}
	} else {
		err = errors.New("services not found in analysis template")
		return OPSMXMetric{}, err
	}
	metric := OPSMXMetric{
		User:                 analysisTemplateData.Data["user"],
		GateUrl:              analysisTemplateData.Data["gateUrl"],
		Application:          analysisTemplateData.Data["application"],
		BaselineStartTime:    analysisTemplateData.Data["baselineStartTime"],
		CanaryStartTime:      analysisTemplateData.Data["canaryStartTime"],
		LifetimeMinutes:      lifetimeMinutes,
		EndTime:              analysisTemplateData.Data["endTime"],
		IntervalTime:         intervalTime,
		Delay:                delay,
		GitOPS:               gitops,
		LookBackType:         analysisTemplateData.Data["lookBackType"],
		GlobalLogTemplate:    analysisTemplateData.Data["globalLogTemplate"],
		GlobalMetricTemplate: analysisTemplateData.Data["globalMetricTemplate"],
		Profile:              analysisTemplateData.Data["profile"],
		Threshold: OPSMXThreshold{
			Pass:     pass,
			Marginal: marginal,
		},
		Services: services.Services,
	}

	return metric, nil
}

func encryptString(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash
}

func getTemplateData(Namespace string, kubeclientset kubernetes.Interface, client http.Client, secretData map[string]string, template string) (string, error) {
	var templateData string
	templates, err := kubeclientset.CoreV1().ConfigMaps(Namespace).Get(context.TODO(), template, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	type templateResponse struct {
		Status  string `json:"status,omitempty"`
		Message string `json:"message,omitempty"`
		Path    string `json:"path,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	var templateCheckSave templateResponse

	if templates.Data["Template"] == "ISDTemplate" {
		if templates.Data["TemplateType"] == "" || templates.Data["Json"] == "" {
			err = errors.New("config map file has missing paramters")
			return "", err
		}

		if !isJSON(templates.Data["Json"]) {
			err = errors.New("invalid template json provided")
			return "", err
		}

		sha1Code := encryptString(templates.Data["Json"])
		templateType := templates.Data["TemplateType"]
		tempLink := fmt.Sprintf(templateApi, sha1Code, templateType, template)
		s := []string{secretData["gateUrl"], tempLink}
		templateUrl := strings.Join(s, "")

		data, err := makeRequest(client, "GET", templateUrl, "", secretData["user"])
		if err != nil {
			return "", err
		}
		var templateVerification bool
		json.Unmarshal(data, &templateVerification)
		templateData = sha1Code

		if !templateVerification {
			data, err = makeRequest(client, "POST", templateUrl, templates.Data["Json"], secretData["user"])
			if err != nil {
				return "", err
			}
			json.Unmarshal(data, &templateCheckSave)
			if templateCheckSave.Error != "" && templateCheckSave.Message != "" {
				errorss := fmt.Sprintf("%v", templateCheckSave.Message)
				err = errors.New(errorss)
				return "", err
			}
		}
	} else {
		err = errors.New("no templates found")
		return "", err
	}
	return templateData, nil
}

func (metric *OPSMXMetric) getDataSecret(Namespace string, kubeclientset kubernetes.Interface, isRun bool) (map[string]string, error) {
	secretData := map[string]string{}
	secretName := defaultSecretName
	if metric.Profile != "" {
		secretName = metric.Profile
	}
	secret, err := kubeclientset.CoreV1().Secrets(Namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	gateUrl := metric.GateUrl
	if gateUrl == "" {
		secretGateurl, ok := secret.Data["gate-url"]
		if !ok {
			err := errors.New("the gate-url is not specified both in the template and in the secret")
			return nil, err
		}
		gateUrl = string(secretGateurl)
	}
	secretData["gateUrl"] = gateUrl

	user := metric.User
	if user == "" {
		secretUser, ok := secret.Data["user"]
		if !ok {
			err := errors.New("the user is not specified both in the template and in the secret")
			return nil, err
		}
		user = string(secretUser)
	}
	secretData["user"] = user

	if !isRun {
		return secretData, nil
	}

	var cdIntegration string
	secretCdIntegration, ok := secret.Data["cd-integration"]
	if !ok {
		err := errors.New("cd-integration is not specified in the secret")
		return nil, err
	} else {
		if string(secretCdIntegration) == "true" {
			cdIntegration = cdIntegrationArgoCD
		} else if string(secretCdIntegration) == "false" {
			cdIntegration = cdIntegrationArgoRollouts
		} else {
			err := errors.New("cd-integration should be either true or false")
			return nil, err
		}
	}
	secretData["cdIntegration"] = cdIntegration

	secretSourceName, ok := secret.Data["source-name"]
	if !ok {
		err := errors.New("source-name is not specified in the secret")
		return nil, err
	}
	secretData["sourceName"] = string(secretSourceName)

	return secretData, nil
}

// Evaluate canaryScore and accordingly set the AnalysisPhase
func evaluateResult(score int, pass int, marginal int) string {
	if score >= pass {
		return "Successful"
	}
	if score < pass && score >= marginal {
		return "Inconclusive"
	}
	return "Failed"
}

// Extract the canaryScore and evaluateResult
func (metric *OPSMXMetric) processResume(data []byte) (string, string, error) {
	var (
		canaryScore string
		result      map[string]interface{}
		finalScore  map[string]interface{}
	)

	if !json.Valid(data) {
		err := errors.New("invalid Response")
		return "", "", err
	}

	json.Unmarshal(data, &result)
	jsonBytes, _ := json.MarshalIndent(result["canaryResult"], "", "   ")
	json.Unmarshal(jsonBytes, &finalScore)
	if finalScore["overallScore"] == nil {
		canaryScore = "0"
	} else {
		canaryScore = fmt.Sprintf("%v", finalScore["overallScore"])
	}
	score, _ := strconv.Atoi(canaryScore)
	Phase := evaluateResult(score, int(metric.Threshold.Pass), int(metric.Threshold.Marginal))
	return Phase, canaryScore, nil
}
