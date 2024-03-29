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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-rollouts/utils/defaults"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const DefaultsErrorTopicsJson = `{
	"errorTopics": [
	  {
		"string": "OnOutOfMemoryError",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "StackOverflowError",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "ClassNotFoundException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "FileNotFoundException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "ArrayIndexOutOfBounds",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "NullPointerException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "StringIndexOutOfBoundsException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "FATAL",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "SEVERE",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "NoClassDefFoundError",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "NoSuchMethodFoundError",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "NumberFormatException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "IllegalArgumentException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "ParseException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "SQLException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "ArithmeticException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "status=404",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "status=500",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "EXCEPTION",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "ERROR",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "WARN",
		"topic": "warn",
		"type": "default"
	  }
	]
  }`

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func logNon0CodeExit(exitcode ExitCode) {
	log.Infof("exiting the pod with status code %d", exitcode)
	os.Exit(int(exitcode))
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
		return "", errors.New("the owner of the pod is not a job")
	}
	return podOwner.Name, nil
}

func checkPatchabilityReturnResources(c *Clients) (ResourceNames, error) {

	podName, ok := os.LookupEnv("MY_POD_NAME")
	if !ok {
		return ResourceNames{}, errors.New("analysisTemplate validation error: environment variable MY_POD_NAME is not set")
	}

	jobName, err := getJobNameFromPod(c, podName)
	if err != nil {
		return ResourceNames{}, err
	}

	log.Println("jobname earlier ", jobName)
	_, err = c.kubeclientset.BatchV1().Jobs(defaults.Namespace()).Patch(context.TODO(), jobName, types.StrategicMergePatchType, []byte(`{}`), metav1.PatchOptions{}, "status")
	if err != nil {
		log.Error("cannot patch to Job")
		return ResourceNames{}, err
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

func makeRequest(client http.Client, requestType string, url string, body string, user string) ([]byte, string, string, error) {
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
		return []byte{}, "", "", err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, "", "", err
	}
	var urlScore string
	var urlToken string
	if strings.Contains(url, "registerCanary") {
		urlScore = res.Header.Get("Location")
		urlToken = res.Header.Get("x-opsmx-report-token")
	}
	return data, urlScore, urlToken, err
}

func (metric *OPSMXMetric) checkISDUrl(c *Clients, opsmxIsdUrl string) error {
	resp, err := c.client.Get(opsmxIsdUrl)
	if err != nil && metric.OpsmxIsdUrl != "" && !strings.Contains(err.Error(), "timeout") {
		errorMsg := fmt.Sprintf("provider config map validation error: incorrect opsmxIsdUrl: %v", opsmxIsdUrl)
		return errors.New(errorMsg)
	} else if err != nil && metric.OpsmxIsdUrl == "" && !strings.Contains(err.Error(), "timeout") {
		errorMsg := fmt.Sprintf("opsmx profile secret validation error: incorrect opsmxIsdUrl: %v", opsmxIsdUrl)
		return errors.New(errorMsg)
	} else if err != nil {
		return errors.New(err.Error())
	} else if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

// Check few conditions pre-analysis
func (metric *OPSMXMetric) basicChecks() error {
	if metric.LifetimeMinutes == 0 && metric.EndTime == "" {
		return errors.New("provider config map validation error: provide either lifetimeMinutes or end time")
	}
	if metric.CanaryStartTime != metric.BaselineStartTime && metric.LifetimeMinutes == 0 {
		return errors.New("provider config map validation error: both canaryStartTime and baselineStartTime should be kept same while using endTime argument for analysis")
	}
	if metric.LifetimeMinutes != 0 && metric.LifetimeMinutes < 3 {
		return errors.New("provider config map validation error: lifetimeMinutes cannot be less than 3 minutes")
	}
	if metric.IntervalTime != 0 && metric.IntervalTime < 3 {
		return errors.New("provider config map validation error: intervalTime cannot be less than 3 minutes")
	}
	if metric.LookBackType != "" && metric.IntervalTime == 0 {
		return errors.New("provider config map validation error: intervalTime should be given along with lookBackType to perform interval analysis")
	}
	return nil
}

// Return epoch values of the specific time provided along with lifetimeMinutes for the Run
func (metric *OPSMXMetric) getTimeVariables() error {

	var canaryStartTime string
	var baselineStartTime string
	tm := time.Now()

	if metric.CanaryStartTime == "" {
		canaryStartTime = fmt.Sprintf("%d", tm.UnixNano()/int64(time.Millisecond))
	} else {
		tsStart, err := time.Parse(time.RFC3339, metric.CanaryStartTime)
		if err != nil {
			errorMsg := fmt.Sprintf("provider config map validation error: Error in parsing canaryStartTime: %v", err)
			return errors.New(errorMsg)
		}
		canaryStartTime = fmt.Sprintf("%d", tsStart.UnixNano()/int64(time.Millisecond))
	}

	if metric.BaselineStartTime == "" {
		baselineStartTime = fmt.Sprintf("%d", tm.UnixNano()/int64(time.Millisecond))
	} else {
		tsStart, err := time.Parse(time.RFC3339, metric.BaselineStartTime)
		if err != nil {
			errorMsg := fmt.Sprintf("provider config map validation error: Error in parsing baselineStartTime: %v", err)
			return errors.New(errorMsg)
		}
		baselineStartTime = fmt.Sprintf("%d", tsStart.UnixNano()/int64(time.Millisecond))
	}

	//If lifetimeMinutes not given calculate using endTime
	if metric.LifetimeMinutes == 0 {
		tsEnd, err := time.Parse(time.RFC3339, metric.EndTime)
		if err != nil {
			errorMsg := fmt.Sprintf("provider config map validation error: Error in parsing endTime: %v", err)
			return errors.New(errorMsg)
		}
		if metric.CanaryStartTime != "" && metric.CanaryStartTime > metric.EndTime {
			err := errors.New("provider config map validation error: canaryStartTime cannot be greater than endTime")
			return err
		}
		tsStart := tm
		if metric.CanaryStartTime != "" {
			tsStart, _ = time.Parse(time.RFC3339, metric.CanaryStartTime)
		}
		tsDifference := tsEnd.Sub(tsStart)
		min, _ := time.ParseDuration(tsDifference.String())
		metric.LifetimeMinutes = int(roundFloat(min.Minutes(), 0))
	}
	metric.BaselineStartTime = baselineStartTime
	metric.CanaryStartTime = canaryStartTime
	return nil
}

func getAnalysisTemplateData(basePath string) (OPSMXMetric, error) {
	path := filepath.Join(basePath, "provider/providerConfig")
	data, err := os.ReadFile(path)
	if err != nil {
		errorMsg := fmt.Sprintf("provider config map validation error: %v\n Action Required: Provider config map has to be mounted on '/etc/config/provider' in AnalysisTemplate and must carry data element 'providerConfig'", err)
		err = errors.New(errorMsg)
		return OPSMXMetric{}, err
	}

	var opsmx OPSMXMetric
	if err := yaml.Unmarshal(data, &opsmx); err != nil {
		errorMsg := fmt.Sprintf("provider config map validation error: %v", err)
		err = errors.New(errorMsg)
		return OPSMXMetric{}, err
	}

	if opsmx.Application == "" {
		opsmx.Application, err = getScopeValues("{{env.APP_NAME}}")
		if err != nil {
			log.Warn("provider config map validation warning: unset environment variable APPName and missing application parameter in the provider config map.")
			log.Info("attempting to retrieve App Name via labels of provider ConfigMap")
		}
	}
	return opsmx, nil
}

func getProviderConfigNameFromJob(c *Clients, r ResourceNames) (string, error) {
	jobValue, err := c.kubeclientset.BatchV1().Jobs(defaults.Namespace()).Get(context.TODO(), r.jobName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var analysisTemplateName string
	for i := range jobValue.Spec.Template.Spec.Volumes {
		if jobValue.Spec.Template.Spec.Volumes[i].ConfigMap.LocalObjectReference.Name != "" {
			analysisTemplateName = jobValue.Spec.Template.Spec.Volumes[i].ConfigMap.LocalObjectReference.Name
			break
		}
	}
	analysisTemplate, err := c.kubeclientset.CoreV1().ConfigMaps(defaults.Namespace()).Get(context.TODO(), analysisTemplateName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return analysisTemplate.ObjectMeta.Labels["argocd.argoproj.io/instance"], nil
}

func generateSHA1(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash
}

func isExists(list []string, item string) bool {
	for _, v := range list {
		if item == v {
			return true
		}
	}
	return false
}

func getTemplateDataYaml(templateFileData []byte, template string, templateType string, ScopeVariables string) ([]byte, error) {
	if templateType == "LOG" {
		var logdata LogTemplateYaml
		if err := yaml.Unmarshal([]byte(templateFileData), &logdata); err != nil {
			errorMessage := fmt.Sprintf("gitops '%s' template config map validation error: %v", template, err)
			return nil, errors.New(errorMessage)
		}
		logdata.TemplateName = template
		logdata.FilterKey = ScopeVariables
		if len(logdata.Tags) >= 1 {
			logdata.TagEnabled = true
		}

		var defaults LogTemplateYaml
		err := json.Unmarshal([]byte(DefaultsErrorTopicsJson), &defaults)
		if err != nil {
			return nil, err
		}

		var defaultErrorString []string
		defaultErrorStringMapType := make(map[string]string)
		for _, items := range defaults.ErrorTopics {
			defaultErrorStringMapType[items.ErrorStrings] = items.Topic
			defaultErrorString = append(defaultErrorString, items.ErrorStrings)
		}

		var errorStringsAvailable []string

		for i, items := range logdata.ErrorTopics {
			errorStringsAvailable = append(errorStringsAvailable, items.ErrorStrings)

			if isExists(defaultErrorString, items.ErrorStrings) {
				if items.Topic == defaultErrorStringMapType[items.ErrorStrings] {
					logdata.ErrorTopics[i].Type = "default"
				} else {
					logdata.ErrorTopics[i].Type = "custom"
				}
			}
		}

		if !logdata.DisableDefaultsErrorTopics {
			log.Info("loading defaults tags for log template")
			for _, items := range defaults.ErrorTopics {
				if !isExists(errorStringsAvailable, items.ErrorStrings) {
					logdata.ErrorTopics = append(logdata.ErrorTopics, items)
				}
			}
		}
		if logdata.ErrorTopics == nil {
			logdata.ErrorTopics = make([]errorTopics, 0)
		}
		log.Info("processed template and converting to json", logdata)
		return json.Marshal(logdata)
	}

	metricStruct, err := processYamlMetrics(templateFileData, template, ScopeVariables)
	if err != nil {
		return nil, err
	}
	return json.Marshal(metricStruct)

}

func getTemplateData(client http.Client, secretData map[string]string, template string, templateType string, basePath string, ScopeVariables string) (string, error) {
	log.Info("processing gitops template", template)
	var templateData string
	templatePath := filepath.Join(basePath, "templates/")
	path := filepath.Join(templatePath, template)
	templateFileData, err := os.ReadFile(path)
	if err != nil {
		errorMsg := fmt.Sprintf("gitops '%s' template config map validation error: %v\n Action Required: Template has to be mounted on '/etc/config/templates' in AnalysisTemplate and must carry data element '%s'", template, err, template)
		err = errors.New(errorMsg)
		return "", err
	}
	log.Info("checking if json or yaml for template ", template)
	if !isJSON(string(templateFileData)) {
		log.Info("template not recognized in json format")
		templateFileData, err = getTemplateDataYaml(templateFileData, template, templateType, ScopeVariables)
		log.Info("json for template ", template, string(templateFileData))
		if err != nil {
			return "", err
		}
	} else {
		checktemplateName := gjson.Get(string(templateFileData), "templateName")
		if checktemplateName.String() == "" {
			errmessage := fmt.Sprintf("gitops '%s' template config map validation error: template name not provided inside json", template)
			return "", errors.New(errmessage)
		}
		if template != checktemplateName.String() {
			errmessage := fmt.Sprintf("gitops '%s' template config map validation error: Mismatch between templateName and data.%s key", template, template)
			return "", errors.New(errmessage)
		}
	}

	sha1Code := generateSHA1(string(templateFileData))
	tempLink := fmt.Sprintf(templateApi, sha1Code, templateType, template)
	s := []string{secretData["opsmxIsdUrl"], tempLink}
	templateUrl := strings.Join(s, "")

	log.Debug("sending a GET request to gitops API")
	data, _, _, err := makeRequest(client, "GET", templateUrl, "", secretData["user"])
	if err != nil {
		return "", err
	}
	var templateVerification bool
	err = json.Unmarshal(data, &templateVerification)
	if err != nil {
		errorMessage := fmt.Sprintf("analysis Error: Expected bool response from gitops verifyTemplate response  Error: %v. Action: Check endpoint given in secret/providerConfig.", err)
		return "", errors.New(errorMessage)
	}
	templateData = sha1Code
	var templateCheckSave map[string]interface{}
	if !templateVerification {
		log.Debug("sending a POST request to gitops API")
		data, _, _, err = makeRequest(client, "POST", templateUrl, string(templateFileData), secretData["user"])
		if err != nil {
			return "", err
		}
		err = json.Unmarshal(data, &templateCheckSave)
		if err != nil {
			return "", err
		}
		log.Debugf("the value of templateCheckSave var is %v", templateCheckSave)
		var errorss string
		if templateCheckSave["errorMessage"] != nil && templateCheckSave["errorMessage"] != "" {
			errorss = fmt.Sprintf("%v", templateCheckSave["errorMessage"])
		} else {
			errorss = fmt.Sprintf("%v", templateCheckSave["error"])
		}
		errorss = strings.Replace(strings.Replace(errorss, "[", "", -1), "]", "", -1)
		if templateCheckSave["status"] != "CREATED" {
			err = fmt.Errorf("gitops '%s' template config map validation error: %s", template, errorss)
			return "", err
		}
	}
	return templateData, nil
}

func (metric *OPSMXMetric) getDataSecret(basePath string) (map[string]string, error) {

	secretData := map[string]string{}
	userPath := filepath.Join(basePath, "secrets/user")
	secretUser, err := os.ReadFile(userPath)
	if err != nil {
		err = errors.New("opsmx profile secret validation error: `user` key not present in the secret file\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'user'")
		return nil, err
	}
	opsmxIsdUrlPath := filepath.Join(basePath, "secrets/opsmxIsdUrl")
	opsmxIsdUrl, err := os.ReadFile(opsmxIsdUrlPath)
	if err != nil {
		err = errors.New("opsmx profile secret validation error: `opsmxIsdUrl` key not present in the secret file\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'opsmxIsdUrl'")
		return nil, err
	}
	sourceNamePath := filepath.Join(basePath, "secrets/sourceName")
	secretsourcename, err := os.ReadFile(sourceNamePath)
	if err != nil {
		err = errors.New("opsmx profile secret validation error: `sourceName` key not present in the secret file\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'sourceName'")
		return nil, err
	}
	cdIntegrationPath := filepath.Join(basePath, "secrets/cdIntegration")
	secretcdintegration, err := os.ReadFile(cdIntegrationPath)
	if err != nil {
		err = errors.New("opsmx profile secret validation error: `cdIntegration` key not present in the secret file\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'cdIntegration'")
		return nil, err
	}

	agentNamePath := filepath.Join(basePath, "secrets/agentName")
	secretagentname, err := os.ReadFile(agentNamePath)
	if err != nil && string(secretcdintegration) == "true" {
		err = errors.New("opsmx profile secret validation error: `agentName` key not present in the secret file\n Action Required: secret file has to be mounted on '/etc/config/secrets' in AnalysisTemplate and must carry data element 'agentName' for 'cdIntegration' as 'true'")
		return nil, err
	}

	opsmxIsdURL := metric.OpsmxIsdUrl
	if opsmxIsdURL == "" {
		opsmxIsdURL = string(opsmxIsdUrl)
	}
	secretData["opsmxIsdUrl"] = opsmxIsdURL

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
		err := errors.New("opsmx profile secret validation error: cdIntegration should be either true or false")
		return nil, err
	}
	secretData["cdIntegration"] = cdIntegration

	secretData["sourceName"] = string(secretsourcename)

	secretData["agentName"] = string(secretagentname)

	return secretData, nil
}

func getScopeValues(scope string) (string, error) {
	splitScope := strings.Split(scope, ",")
	for i, items := range splitScope {
		if strings.Contains(items, "{{env.") {
			extrctVal := strings.Split(items, "{{env.")
			extractkey := strings.Split(extrctVal[1], "}}")
			podName, ok := os.LookupEnv(extractkey[0])
			if !ok {
				err := fmt.Sprintf("analysisTemplate validation error: environment variable %s not set", extractkey[0])
				return "", errors.New(err)
			}
			old := fmt.Sprintf("{{env.%s}}", extractkey[0])
			testresult := strings.Replace(items, old, podName, 1)
			splitScope[i] = testresult
		}
	}
	scopeValue := strings.Join(splitScope, ",")
	return scopeValue, nil
}

func (metric *OPSMXMetric) generatePayload(c *Clients, secretData map[string]string, basePath string) (string, error) {
	var intervalTime string
	if metric.IntervalTime != 0 {
		intervalTime = fmt.Sprintf("%d", metric.IntervalTime)
	}

	var opsmxdelay string
	if metric.Delay != 0 {
		opsmxdelay = fmt.Sprintf("%d", metric.Delay)
	}
	var services []string
	//Generate the payload
	payload := jobPayload{
		Application: metric.Application,
		SourceName:  secretData["sourceName"],
		SourceType:  secretData["cdIntegration"],
		AgentName:   secretData["agentName"],
		CanaryConfig: canaryConfig{
			LifetimeMinutes: fmt.Sprintf("%d", metric.LifetimeMinutes),
			LookBackType:    metric.LookBackType,
			IntervalTime:    intervalTime,
			Delays:          opsmxdelay,
			CanaryHealthCheckHandler: canaryHealthCheckHandler{
				MinimumCanaryResultScore: fmt.Sprintf("%d", metric.Pass),
			},
			CanarySuccessCriteria: canarySuccessCriteria{
				CanaryResultScore: fmt.Sprintf("%d", metric.Pass),
			},
		},
		CanaryDeployments: []canaryDeployments{},
	}
	if metric.Services != nil || len(metric.Services) != 0 {
		deployment := canaryDeployments{
			BaselineStartTimeMs: metric.BaselineStartTime,
			CanaryStartTimeMs:   metric.CanaryStartTime,
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
			if isExists(services, serviceName) {
				errorMsg := fmt.Sprintf("provider config map validation error: serviceName '%s' mentioned in provider Config exists more than once", serviceName)
				return "", errors.New(errorMsg)
			}
			services = append(services, serviceName)
			gateName := fmt.Sprintf("gate%d", i+1)
			if item.LogScopeVariables == "" && item.BaselineLogScope != "" || item.LogScopeVariables == "" && item.CanaryLogScope != "" {
				errorMsg := fmt.Sprintf("provider config map validation error: missing log Scope placeholder for the provided baseline/canary of service '%s'", serviceName)
				err := errors.New(errorMsg)
				if err != nil {
					return "", err
				}
			}
			//For Log Analysis is to be added in analysis-run
			if item.LogScopeVariables != "" {
				//Check if no baseline or canary
				if item.BaselineLogScope != "" && item.CanaryLogScope == "" {
					errorMsg := fmt.Sprintf("provider config map validation error: missing canary for log analysis of service '%s'", serviceName)
					err := errors.New(errorMsg)
					if err != nil {
						return "", err
					}
				}
				//Check if the number of placeholders provided dont match
				if len(strings.Split(item.LogScopeVariables, ",")) != len(strings.Split(item.BaselineLogScope, ",")) || len(strings.Split(item.LogScopeVariables, ",")) != len(strings.Split(item.CanaryLogScope, ",")) {
					errorMsg := fmt.Sprintf("provider config map validation error: mismatch in number of log scope variables and baseline/canary log scope of service '%s'", serviceName)
					err := errors.New(errorMsg)
					if err != nil {
						return "", err
					}
				}
				if item.LogTemplateName == "" && metric.GlobalLogTemplate == "" {
					errorMsg := fmt.Sprintf("provider config map validation error: provide either a service specific log template or global log template for service '%s'", serviceName)
					err := errors.New(errorMsg)
					if err != nil {
						return "", err
					}
				}

				baslineLogScope, errors := getScopeValues(item.BaselineLogScope)
				if errors != nil {
					return "", errors
				}
				//Add mandatory field for baseline
				deployment.Baseline.Log[serviceName] = map[string]string{
					item.LogScopeVariables: baslineLogScope,
					"serviceGate":          gateName,
				}

				canaryLogScope, errors := getScopeValues(item.CanaryLogScope)
				if errors != nil {
					return "", errors
				}
				//Add mandatory field for canary
				deployment.Canary.Log[serviceName] = map[string]string{
					item.LogScopeVariables: canaryLogScope,
					"serviceGate":          gateName,
				}

				var tempName string
				tempName = item.LogTemplateName
				if item.LogTemplateName == "" {
					tempName = metric.GlobalLogTemplate
				}

				//Add service specific templateName
				deployment.Baseline.Log[serviceName]["template"] = tempName
				deployment.Canary.Log[serviceName]["template"] = tempName

				var templateData string
				var err error
				if metric.GitOPS && item.LogTemplateVersion == "" {
					templateData, err = getTemplateData(c.client, secretData, tempName, "LOG", basePath, item.LogScopeVariables)
					if err != nil {
						return "", err
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
				errorMsg := fmt.Sprintf("provider config map validation error: missing metric Scope placeholder for the provided baseline/canary of service '%s'", serviceName)
				err := errors.New(errorMsg)
				if err != nil {
					return "", err
				}
			}
			//For metric analysis is to be added in analysis-run
			if item.MetricScopeVariables != "" {
				//Check if no baseline or canary
				if item.BaselineMetricScope == "" || item.CanaryMetricScope == "" {
					errorMsg := fmt.Sprintf("provider config map validation error: missing baseline/canary for metric analysis of service '%s'", serviceName)
					err := errors.New(errorMsg)
					if err != nil {
						return "", err
					}
				}
				//Check if the number of placeholders provided dont match
				if len(strings.Split(item.MetricScopeVariables, ",")) != len(strings.Split(item.BaselineMetricScope, ",")) || len(strings.Split(item.MetricScopeVariables, ",")) != len(strings.Split(item.CanaryMetricScope, ",")) {
					errorMsg := fmt.Sprintf("provider config map validation error: mismatch in number of metric scope variables and baseline/canary metric scope of service '%s'", serviceName)
					err := errors.New(errorMsg)
					if err != nil {
						return "", err
					}
				}
				if item.MetricTemplateName == "" && metric.GlobalMetricTemplate == "" {
					errorMsg := fmt.Sprintf("provider config map validation error: provide either a service specific metric template or global metric template for service: %s", serviceName)
					err := errors.New(errorMsg)
					if err != nil {
						return "", err
					}
				}

				baselineMetricScope, errors := getScopeValues(item.BaselineMetricScope)
				if errors != nil {
					return "", errors
				}
				//Add mandatory field for baseline
				deployment.Baseline.Metric[serviceName] = map[string]string{
					item.MetricScopeVariables: baselineMetricScope,
					"serviceGate":             gateName,
				}

				canaryMetricScope, errors := getScopeValues(item.CanaryMetricScope)
				if errors != nil {
					return "", errors
				}
				//Add mandatory field for canary
				deployment.Canary.Metric[serviceName] = map[string]string{
					item.MetricScopeVariables: canaryMetricScope,
					"serviceGate":             gateName,
				}

				var tempName string
				tempName = item.MetricTemplateName
				if item.MetricTemplateName == "" {
					tempName = metric.GlobalMetricTemplate
				}

				//Add templateName
				deployment.Baseline.Metric[serviceName]["template"] = tempName
				deployment.Canary.Metric[serviceName]["template"] = tempName

				var templateData string
				var err error
				if metric.GitOPS && item.MetricTemplateVersion == "" {
					templateData, err = getTemplateData(c.client, secretData, tempName, "METRIC", basePath, item.MetricScopeVariables)
					if err != nil {
						return "", err
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
				err := errors.New("provider config map validation error: at least one of log or metric context must be provided")
				if err != nil {
					return "", err
				}
			}
		}
		payload.CanaryDeployments = append(payload.CanaryDeployments, deployment)
	} else {
		//Check if no services were provided
		err := errors.New("provider config map validation error: no services provided")
		return "", err
	}
	buffer, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(buffer), err
}

// Evaluate canaryScore and accordingly set the AnalysisPhase
func evaluateResult(score int, pass int) string {
	if score >= pass {
		return "Successful"
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

	err := json.Unmarshal(data, &result)
	if err != nil {
		errorMessage := fmt.Sprintf("analysis Error: Error in post processing canary Response. Error: %v", err)
		return "", "", errors.New(errorMessage)
	}
	jsonBytes, _ := json.MarshalIndent(result["canaryResult"], "", "   ")
	err = json.Unmarshal(jsonBytes, &finalScore)
	if err != nil {
		return "", "", err
	}
	if finalScore["overallScore"] == nil {
		canaryScore = "0"
	} else {
		canaryScore = fmt.Sprintf("%v", finalScore["overallScore"])
	}

	var score int
	// var err error
	if strings.Contains(canaryScore, ".") {
		floatScore, err := strconv.ParseFloat(canaryScore, 64)
		if err != nil {
			return "", "", err
		}
		score = int(roundFloat(floatScore, 0))
	} else {
		score, err = strconv.Atoi(canaryScore)
		if err != nil {
			return "", "", err
		}
	}

	Phase := evaluateResult(score, int(metric.Pass))
	return Phase, fmt.Sprintf("%v", score), nil
}
