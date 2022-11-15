package main

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	AnalysisPhasePending      = "Pending"
	AnalysisPhaseRunning      = "Running"
	AnalysisPhaseSuccessful   = "Successful"
	AnalysisPhaseFailed       = "Failed"
	AnalysisPhaseError        = "Error"
	AnalysisPhaseInconclusive = "Inconclusive"
)

type Clients struct {
	kubeclientset kubernetes.Interface
	client        http.Client
}

type Conditions struct {
	Message       string      `json:"message,omitempty"`
	Type          string      `json:"type,omitempty"`
	Status        string      `json:"status,omitempty"`
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
}

type Status struct {
	Conditions *[]Conditions `json:"conditions,omitempty"`
}

type JobStatus struct {
	Status Status `json:"status,omitempty"`
}

type ResourceNames struct {
	podName string
	jobName string
}

type CanaryDetails struct {
	jobName   string
	canaryId  string
	reportUrl string
	value     string
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
	Pass                 int            `yaml:"passScore"`
	Marginal             int            `yaml:"marginalScore"`
	Services             []OPSMXService `yaml:"serviceList,omitempty"`
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
