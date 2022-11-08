package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	if metric.Pass <= metric.Marginal {
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

func getAnalysisTemplateData(template string) (OPSMXMetric, error) {
	path := fmt.Sprintf("/etc/config/provider/%s", template)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return OPSMXMetric{}, err
	}

	var opsmx OPSMXMetric
	if err := yaml.Unmarshal(data, &opsmx); err != nil {
		return OPSMXMetric{}, err
	}
	return opsmx, nil
}

func encryptString(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash
}

func getTemplateData(client http.Client, secretData map[string]string, template string, templateType string) (string, error) {
	var templateData string
	path := fmt.Sprintf("/etc/config/templates/%s", template)
	templateFileData, err := ioutil.ReadFile(path)
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

	if !isJSON(string(templateFileData)) {
		err = errors.New("invalid template json provided")
		return "", err
	}

	sha1Code := encryptString(string(templateFileData))
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
		data, err = makeRequest(client, "POST", templateUrl, string(templateFileData), secretData["user"])
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
	return templateData, nil
}

func (metric *OPSMXMetric) getDataSecret() (map[string]string, error) {
	secretData := map[string]string{}
	userPath := "/etc/config/secrets/data/user"
	gateUrlPath := "/etc/config/secrets/data/gate-url"
	sourceNamePath := "/etc/config/secrets/data/source-name"
	cdIntegrationPath := "/etc/config/secrets/data/cd-integration"

	secretUser, err := ioutil.ReadFile(userPath)
	if err != nil {
		return nil, err
	}

	secretGateUrl, err := ioutil.ReadFile(gateUrlPath)
	if err != nil {
		return nil, err
	}

	secretsourcename, err := ioutil.ReadFile(sourceNamePath)
	if err != nil {
		return nil, err
	}

	secretcdintegration, err := ioutil.ReadFile(cdIntegrationPath)
	if err != nil {
		return nil, err
	}

	gateUrl := metric.GateUrl
	if gateUrl == "" {
		gateUrl = string(secretGateUrl)
	}
	secretData["gateUrl"] = gateUrl

	user := metric.User
	if user == "" {
		user = string(secretUser)
	}
	secretData["user"] = user

	var cdIntegration string
	if string(secretcdintegration) == "true" {
		cdIntegration = cdIntegrationArgoCD
	} else if string(secretcdintegration) == "false" {
		cdIntegration = cdIntegrationArgoRollouts
	} else {
		err := errors.New("cd-integration should be either true or false")
		return nil, err
	}
	secretData["cdIntegration"] = cdIntegration

	secretData["sourceName"] = string(secretsourcename)

	return secretData, nil
}

func getScopeValues(scope string) string {
	splitScope := strings.Split(scope, ",")
	for i, items := range splitScope {
		if strings.Contains(items, "{{env.") {
			extrctVal := strings.Split(items, "{{env.")
			extractkey := strings.Split(extrctVal[1], "}}")
			podName := os.Getenv(extractkey[0])
			old := fmt.Sprintf("{{env.%s}}", extractkey[0])
			testresult := strings.Replace(items, old, podName, 1)
			splitScope[i] = testresult
		}
	}
	scopeValue := strings.Join(splitScope, ",")
	return scopeValue
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
	Phase := evaluateResult(score, int(metric.Pass), int(metric.Marginal))
	return Phase, canaryScore, nil
}
