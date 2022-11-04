package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"

	"gopkg.in/yaml.v2"

	"math"

	"net/http"
	"net/url"

	"fmt"
	"io"
	"strconv"
	"strings"

	"time"

	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

type Provider struct {
	kubeclientset kubernetes.Interface
	client        http.Client
}

type OPSMXMetric struct {
	User                 string         `yaml:"user,omitempty"`
	GateUrl              string         `yaml:"gateUrl,omitempty"`
	Application          string         `yaml:"application"`
	BaselineStartTime    string         `yaml:"baselineStartTime,omitempty"`
	CanaryStartTime      string         `yaml:"canaryStartTime,omitempty"`
	LifetimeMinutes      int            `yaml:"lifetimeMinutes,omitempty"`
	EndTime              string         `yaml:"endTime,omitempty"`
	GlobalLogTemplate    string         `yaml:"globalLogTemplate,omitempty"`
	GlobalMetricTemplate string         `yaml:"globalMetricTemplate,omitempty"`
	Threshold            OPSMXThreshold `yaml:"threshold"`
	Services             []OPSMXService `yaml:"services,omitempty"`
	Profile              string         `yaml:"profile,omitempty"`
	IntervalTime         int            `yaml:"intervalTime,omitempty"`
	LookBackType         string         `yaml:"lookBackType,omitempty"`
	Delay                int            `yaml:"delay,omitempty"`
	GitOPS               bool           `yaml:"gitops,omitempty"`
}

type OPSMXService struct {
	LogTemplateName       string `yaml:"logTemplateName,omitempty"`
	LogTemplateVersion    string `yaml:"logTemplateVersion,omitempty"`
	MetricTemplateName    string `yaml:"metricTemplateName,omitempty"`
	MetricTemplateVersion string `yaml:"metricTemplateVersion,omitempty"`
	LogScopeVariables     string `yaml:"logScopeVariables,omitempty"`
	BaselineLogScope      string `yaml:"baselineLogScope,omitempty"`
	CanaryLogScope        string `yaml:"canaryLogScope,omitempty"`
	MetricScopeVariables  string `yaml:"metricScopeVariables,omitempty"`
	BaselineMetricScope   string `yaml:"baselineMetricScope,omitempty"`
	CanaryMetricScope     string `yaml:"canaryMetricScope,omitempty"`
	ServiceName           string `yaml:"serviceName,omitempty"`
}

type OPSMXThreshold struct {
	Pass     int `yaml:"pass"`
	Marginal int `yaml:"marginal"`
}

type jobPayload struct {
	Application       string              `json:"application"`
	SourceName        string              `json:"sourceName"`
	SourceType        string              `json:"sourceType"`
	CanaryConfig      canaryConfig        `json:"canaryConfig"`
	CanaryDeployments []canaryDeployments `json:"canaryDeployments"`
}

type canaryConfig struct {
	LifetimeMinutes          string                   `json:"lifetimeMinutes"`
	LookBackType             string                   `json:"lookBackType,omitempty"`
	IntervalTime             string                   `json:"interval,omitempty"`
	Delays                   string                   `json:"delay,omitempty"`
	CanaryHealthCheckHandler canaryHealthCheckHandler `json:"canaryHealthCheckHandler"`
	CanarySuccessCriteria    canarySuccessCriteria    `json:"canarySuccessCriteria"`
}

type canaryHealthCheckHandler struct {
	MinimumCanaryResultScore string `json:"minimumCanaryResultScore"`
}

type canarySuccessCriteria struct {
	CanaryResultScore string `json:"canaryResultScore"`
}

type canaryDeployments struct {
	CanaryStartTimeMs   string     `json:"canaryStartTimeMs"`
	BaselineStartTimeMs string     `json:"baselineStartTimeMs"`
	Canary              *logMetric `json:"canary,omitempty"`
	Baseline            *logMetric `json:"baseline,omitempty"`
}
type logMetric struct {
	Log    map[string]map[string]string `json:"log,omitempty"`
	Metric map[string]map[string]string `json:"metric,omitempty"`
}

func isJSON(s string) bool {
	var j map[string]interface{}
	if err := json.Unmarshal([]byte(s), &j); err != nil {
		return false
	}
	return true
}

func check(err error) {
	if err != nil {
		panic(err)
	}
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
		return "Analysis Successful"
	}
	if score < pass && score >= marginal {
		return "Analysis Inconclusive"
	}
	return "Analysis Failed"
}

// Extract the canaryScore and evaluateResult
func (metric *OPSMXMetric) processResume(data []byte) (string, error) {
	var (
		canaryScore string
		result      map[string]interface{}
		finalScore  map[string]interface{}
	)

	if !json.Valid(data) {
		err := errors.New("invalid Response")
		return "", err
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
	if Phase == "Failed" && metric.LookBackType != "" {
		return fmt.Sprintf("Interval Analysis Failed at intervalNo. %s", finalScore["intervalNo"]), nil
	}
	return Phase, nil
}

func runAnalysis(c *Clients, r ResourceNames) error{
	p := Provider{}
	metric, err := getAnalysisTemplateData("a", "argocd", p.kubeclientset)
	check(err)
	err = metric.basicChecks()
	check(err)
	secretData, err := metric.getDataSecret("argocd", p.kubeclientset, true)
	check(err)
	canaryurl, err := url.JoinPath(secretData["gateUrl"], v5configIdLookupURLFormat)
	check(err)
	//Get the epochs for Time variables and the lifetimeMinutes
	canaryStartTime, baselineStartTime, lifetimeMinutes, err := getTimeVariables(metric.BaselineStartTime, metric.CanaryStartTime, metric.EndTime, metric.LifetimeMinutes)
	check(err)

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
				MinimumCanaryResultScore: fmt.Sprintf("%d", metric.Threshold.Marginal),
			},
			CanarySuccessCriteria: canarySuccessCriteria{
				CanaryResultScore: fmt.Sprintf("%d", metric.Threshold.Pass),
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
				check(err)
			}
			//For Log Analysis is to be added in analysis-run
			if item.LogScopeVariables != "" {
				//Check if no baseline or canary
				if item.BaselineLogScope != "" && item.CanaryLogScope == "" {
					err := errors.New("missing canary for log analysis")
					check(err)
				}
				//Check if the number of placeholders provided dont match
				if len(strings.Split(item.LogScopeVariables, ",")) != len(strings.Split(item.BaselineLogScope, ",")) || len(strings.Split(item.LogScopeVariables, ",")) != len(strings.Split(item.CanaryLogScope, ",")) {
					err := errors.New("mismatch in number of log scope variables and baseline/canary log scope")
					check(err)
				}
				if item.LogTemplateName == "" && metric.GlobalLogTemplate == "" {
					err := errors.New("provide either a service specific log template or global log template")
					check(err)
				}
				//Add mandatory field for baseline
				deployment.Baseline.Log[serviceName] = map[string]string{
					item.LogScopeVariables: item.BaselineLogScope,
					"serviceGate":          gateName,
				}
				//Add mandatory field for canary
				deployment.Canary.Log[serviceName] = map[string]string{
					item.LogScopeVariables: item.CanaryLogScope,
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
					templateData, err = getTemplateData("argocd", p.kubeclientset, p.client, secretData, tempName)
					check(err)
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
				check(err)
			}
			//For metric analysis is to be added in analysis-run
			if item.MetricScopeVariables != "" {
				//Check if no baseline or canary
				if item.BaselineMetricScope == "" || item.CanaryMetricScope == "" {
					err := errors.New("missing baseline/canary for metric analysis")
					check(err)
				}
				//Check if the number of placeholders provided dont match
				if len(strings.Split(item.MetricScopeVariables, ",")) != len(strings.Split(item.BaselineMetricScope, ",")) || len(strings.Split(item.MetricScopeVariables, ",")) != len(strings.Split(item.CanaryMetricScope, ",")) {
					err := errors.New("mismatch in number of metric scope variables and baseline/canary metric scope")
					check(err)
				}
				if item.MetricTemplateName == "" && metric.GlobalMetricTemplate == "" {
					err := errors.New("provide either a service specific metric template or global metric template")
					check(err)
				}
				//Add mandatory field for baseline
				deployment.Baseline.Metric[serviceName] = map[string]string{
					item.MetricScopeVariables: item.BaselineMetricScope,
					"serviceGate":             gateName,
				}
				//Add mandatory field for canary
				deployment.Canary.Metric[serviceName] = map[string]string{
					item.MetricScopeVariables: item.CanaryMetricScope,
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
					templateData, err = getTemplateData("argocd", p.kubeclientset, p.client, secretData, tempName)
					check(err)
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
				check(err)
			}
		}
		payload.CanaryDeployments = append(payload.CanaryDeployments, deployment)
	} else {
		//Check if no services were provided
		err = errors.New("no services provided")
		check(err)
	}
	buffer, err := json.Marshal(payload)
	check(err)

	data, err := makeRequest(p.client, "POST", canaryurl, string(buffer), secretData["user"])
	check(err)
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
		check(err)
	}
	scoreURL, _ := url.JoinPath(secretData["gateUrl"], scoreUrlFormat, canary.CanaryId.String())

	data, err = makeRequest(p.client, "GET", scoreURL, "", secretData["user"])
	check(err)

	var status map[string]interface{}
	var reportUrlJson map[string]interface{}

	json.Unmarshal(data, &status)
	jsonBytes, _ := json.MarshalIndent(status["canaryResult"], "", "   ")
	json.Unmarshal(jsonBytes, &reportUrlJson)
	reportUrl := reportUrlJson["canaryReportURL"]
	fmt.Printf("%v", reportUrl)

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
			data, err = makeRequest(p.client, "GET", scoreURL, "", secretData["user"])
			check(err)
		}
	}
	//if run is cancelled mid-run
	if status["status"] == "CANCELLED" {
		fmt.Printf("Analysis Cancelled")
	} else {
		//POST-Run process
		measurement, err := metric.processResume(data)
		check(err)
		fmt.Println(measurement)
	}
	os.Exit(0)
}


// var reportUrlJson map[string]interface{}
// jsonBytes, _ := json.MarshalIndent(status["canaryResult"], "", "   ")
// json.Unmarshal(jsonBytes, &reportUrlJson)
// reportUrl := reportUrlJson["canaryReportURL"]

// cd := CanaryDetails{
// 	jobName: r.jobName,
// 	metricName: ar.Spec.Metrics[0].Name,
// 	canaryId: stringifiedCanaryId,
// 	gateUrl: "Gate URL goes here",
// 	reportUrl: fmt.Sprintf("%s", reportUrl),
// 	phase: "Running",
// }
// patchCanaryDetails(c,ctx,r.analysisRunName,cd)

// /// Polling the score API
// time.Sleep(60 * time.Second)


// phase := "Failed"
// if phase ==  AnalysisPhaseSuccessful{

// fs := CanaryDetails{
// 	jobName: r.jobName,
// 	metricName: ar.Spec.Metrics[0].Name,
// 	canaryId: stringifiedCanaryId,
// 	gateUrl: "Gate URL goes here",
// 	reportUrl: fmt.Sprintf("%s", reportUrl),
// 	phase: "Running",
// 	value: "96",
// }
// patchFinalStatus(c,ctx,r.analysisRunName,fs)
// }

// if phase ==  AnalysisPhaseFailed{

// fs := CanaryDetails{
// 	jobName: r.jobName,
// 	metricName: ar.Spec.Metrics[0].Name,
// 	canaryId: stringifiedCanaryId,
// 	gateUrl: "Gate URL goes here",
// 	reportUrl: fmt.Sprintf("%s", reportUrl),
// 	phase: "Running",
// 	value: "40",
// }
// patchFailedInconclusive(c,ctx,r.analysisRunName,phase,fs)



// }

// if phase == AnalysisPhaseInconclusive{	

// fs := CanaryDetails{
// 	jobName: r.jobName,
// 	metricName: ar.Spec.Metrics[0].Name,
// 	canaryId: stringifiedCanaryId,
// 	gateUrl: "Gate URL goes here",
// 	reportUrl: fmt.Sprintf("%s", reportUrl),
// 	phase: "Running",
// 	value: "70",
// }
// patchFailedInconclusive(c,ctx,r.analysisRunName,phase,fs)
// }

// return nil
