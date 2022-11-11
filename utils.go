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
	"k8s.io/apimachinery/pkg/types"
)


func logErrorExit1(err error) {
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func getJobNameFromPod(p *Clients, podName string) (string, error) {
	ns := defaults.Namespace()
	ctx := context.TODO()
	pod, err := p.kubeclientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	podOwner := pod.OwnerReferences[0]
	if podOwner.Kind != "Job" {
		return "", errors.New("The owner of the Pod is not a Job")
	}
	return podOwner.Name, nil
}

func checkPatchabilityReturnResources(c *Clients) (ResourceNames, error) {

	podName, ok := os.LookupEnv("MY_POD_NAME")
	if !ok {
		return *new(ResourceNames), errors.New("Environment variable MY_POD_NAME not set")
	}

	jobName, err := getJobNameFromPod(c, podName)
	if err != nil {
		return *new(ResourceNames), err
	}

	_, err = c.kubeclientset.BatchV1().Jobs(defaults.Namespace()).Patch(context.TODO(), jobName, types.StrategicMergePatchType, []byte(`{}`), metav1.PatchOptions{}, "status")
	if err != nil {
		log.Error("Cannot patch to Job")
		return *new(ResourceNames), err
	}
	resourceNames := ResourceNames{
		podName: podName,
		jobName: jobName,
	}
	return resourceNames, nil
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

func makeRequest(client http.Client, requestType string, url string, body string, user string) ([]byte, string, error) {
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
		return []byte{}, "", err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, "", err
	}
	var urlScore string
	if strings.Contains(url, "registerCanary") {
		urlScore = res.Header.Get("Location")
	}
	return data, urlScore, err
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

func getAnalysisTemplateData() (OPSMXMetric, error) {
	path := "/etc/config/provider/providerConfig"
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

	data, _, err := makeRequest(client, "GET", templateUrl, "", secretData["user"])
	if err != nil {
		return "", err
	}
	var templateVerification bool
	json.Unmarshal(data, &templateVerification)
	templateData = sha1Code

	if !templateVerification {
		data, _, err = makeRequest(client, "POST", templateUrl, string(templateFileData), secretData["user"])
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
	userPath := "/etc/config/secrets/user"
	gateUrlPath := "/etc/config/secrets/gate-url"
	sourceNamePath := "/etc/config/secrets/source-name"
	cdIntegrationPath := "/etc/config/secrets/cd-integration"

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

	var score int
	var err error
	if strings.Contains(canaryScore, ".") {
		floatScore, err := strconv.ParseFloat(canaryScore, 64)
		score = int(roundFloat(floatScore, 0))
		if err != nil {
			return "", "", err
		}
	} else {
		score, err = strconv.Atoi(canaryScore)
		if err != nil {
			return "", "", err
		}
	}

	Phase := evaluateResult(score, int(metric.Pass), int(metric.Marginal))
	return Phase, canaryScore, nil
}
