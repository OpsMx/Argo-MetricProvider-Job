package main

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	AnalysisPhasePending    = "Pending"
	AnalysisPhaseRunning    = "Running"
	AnalysisPhaseSuccessful = "Successful"
	AnalysisPhaseFailed     = "Failed"
	AnalysisPhaseError      = "Error"
)

type ExitCode int

const (
	ReturnCodeSuccess ExitCode = iota
	ReturnCodeError
	ReturnCodeFailed
	ReturnCodeInconclusive
	ReturnCodeCancelled
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
	user      string
	jobName   string
	canaryId  string
	reportUrl string
	value     string
	ReportId  string
}

type OPSMXMetric struct {
	User                 string         `yaml:"user,omitempty"`
	OpsmxIsdUrl          string         `yaml:"opsmxIsdUrl,omitempty"`
	Application          string         `yaml:"application"`
	BaselineStartTime    string         `yaml:"baselineStartTime,omitempty"`
	CanaryStartTime      string         `yaml:"canaryStartTime,omitempty"`
	LifetimeMinutes      int            `yaml:"lifetimeMinutes,omitempty"`
	EndTime              string         `yaml:"endTime,omitempty"`
	GlobalLogTemplate    string         `yaml:"globalLogTemplate,omitempty"`
	GlobalMetricTemplate string         `yaml:"globalMetricTemplate,omitempty"`
	Pass                 int            `yaml:"passScore"`
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

type LogTemplateYaml struct {
	DefaultsErrorTopics  bool          `yaml:"disableDefaultErrorTopics" json:"-"`
	TemplateName         string        `yaml:"templateName" json:"templateName"`
	FilterKey            string        `yaml:"filterKey" json:"filterKey"`
	TagEnabled           bool          `yaml:"-" json:"tagEnabled"`
	MonitoringProvider   string        `yaml:"monitoringProvider" json:"monitoringProvider"`
	AccountName          string        `yaml:"accountName" json:"accountName"`
	ScoringAlgorithm     string        `yaml:"scoringAlgorithm" json:"scoringAlgorithm"`
	Index                string        `yaml:"index,omitempty" json:"index,omitempty"`
	ResponseKeywords     string        `yaml:"responseKeywords" json:"responseKeywords"`
	ContextualCluster    bool          `yaml:"contextualCluster,omitempty" json:"contextualCluster,omitempty"`
	ContextualWindowSize int           `yaml:"contextualWindowSize,omitempty" json:"contextualWindowSize,omitempty"`
	InfoScoring          bool          `yaml:"infoScoring,omitempty" json:"infoScoring,omitempty"`
	RegExFilter          bool          `yaml:"regExFilter,omitempty" json:"regExFilter,omitempty"`
	RegExResponseKey     string        `yaml:"regExResponseKey,omitempty" json:"regExResponseKey,omitempty"`
	RegularExpression    string        `yaml:"regularExpression,omitempty" json:"regularExpression,omitempty"`
	AutoBaseline         bool          `yaml:"autoBaseline,omitempty" json:"autoBaseline,omitempty"`
	Sensitivity          string        `yaml:"sensitivity,omitempty" json:"sensitivity,omitempty"`
	StreamID             string        `yaml:"streamId,omitempty" json:"streamId,omitempty"`
	Tags                 []customTags  `yaml:"tags" json:"tags,omitempty"`
	ErrorTopics          []errorTopics `yaml:"errorTopics" json:"errorTopics"`
}

type customTags struct {
	ErrorStrings string `yaml:"errorString" json:"string"`
	Tag          string `yaml:"tag" json:"tag"`
}

type errorTopics struct {
	ErrorStrings string `yaml:"errorString" json:"string"`
	Topic        string `yaml:"topic" json:"topic"`
	Type         string `yaml:"-" json:"type"`
}
