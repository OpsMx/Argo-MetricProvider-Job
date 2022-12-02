package main

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Metrics struct {
	MetricType            string  `yaml:"metricType" json:"metricType,omitempty"`
	MetricWeight          float64 `yaml:"metricWeight" json:"metricWeight,omitempty"`
	NanStrategy           string  `yaml:"nanStrategy" json:"nanStrategy,omitempty"`
	AccountName           string  `yaml:"accountName" json:"accountName,omitempty"`
	RiskDirection         string  `yaml:"riskDirection" json:"riskDirection,omitempty"`
	CustomThresholdHigher int     `yaml:"customThresholdHigher" json:"customThresholdHigher,omitempty"`
	Name                  string  `yaml:"name" json:"name,omitempty"`
	Criticality           string  `yaml:"criticality" json:"criticality,omitempty"`
	CustomThresholdLower  int     `yaml:"customThresholdLower" json:"customThresholdLower,omitempty"`
	Watchlist             bool    `yaml:"watchlist" json:"watchlist"`
}

type Groups struct {
	Metrics []Metrics `yaml:"metrics" json:"metrics"`
	Group   string    `yaml:"group" json:"group,omitempty"`
}

type Data struct {
	PercentDiffThreshold string   `yaml:"percent_diff_threshold" json:"percent_diff_threshold,omitempty"`
	IsNormalize          bool     `yaml:"isNormalize" json:"isNormalize"`
	Groups               []Groups `yaml:"groups" json:"groups"`
}
type MetricISDTemplate struct {
	FilterKey        string `yaml:"filterKey" json:"filterKey,omitempty"`
	AccountName      string `yaml:"accountName" json:"accountName,omitempty"`
	Data             Data   `yaml:"metricTemplateSetup" json:"data"`
	TemplateName     string `yaml:"templateName" json:"templateName"`
	AdvancedProvider string `yaml:"advancedProvider" json:"advancedProvider"`
}

func (m *MetricISDTemplate) setAccountName(templateName string) error {
	var isAccountNameSet bool
	if m.AccountName == "" {
		errMsg := fmt.Sprintf("metric template %s does not have the accountName field defined at the level of the template", templateName)
		return errors.New(errMsg)
	}
	for _, metric := range m.Data.Groups {
		if metric.Metrics[0].AccountName != "" {
			isAccountNameSet = true
		}
		metric.Metrics[0].AccountName = m.AccountName
	}
	if isAccountNameSet {
		log.Warnf("accountName field has been defined at the level of individual metrics for some of the groups for template %s, they will be overriden by %s", templateName, m.AccountName)
	}
	m.AccountName = ""
	return nil
}

func (m *MetricISDTemplate) setTemplateName(templateName string) {
	if m.TemplateName != "" && m.TemplateName != templateName {
		log.Warnf("the templateName field has been defined in the metric template %s, it will be overriden", templateName)
	}
	m.TemplateName = templateName
}

func (m *MetricISDTemplate) setFilterKey(templateName string, metricScopeVariables string) {
	if m.FilterKey != "" && m.FilterKey != metricScopeVariables {
		log.Warnf("the filterKey field has been defined in the metric template %s, it will be overriden by %s", templateName, metricScopeVariables)
	}
	m.FilterKey = metricScopeVariables
}

func (m *MetricISDTemplate) checkMetricTemplateErrors(templateName string) error {
	//TODO- Extend it further after inputs from Java

	//check for groups array
	if len(m.Data.Groups) == 0 {
		errMsg := fmt.Sprintf("metric template %s does not have any members defined for the groups field", templateName)
		return errors.New(errMsg)
	}
	return nil
}

func processYamlMetrics(templateData []byte, templateName string, scopeVariables string) (MetricISDTemplate, error) {
	metric := MetricISDTemplate{}
	err := yaml.Unmarshal(templateData, &metric)
	if err != nil {
		return MetricISDTemplate{}, err
	}

	/*if err = metric.setAccountName(templateName); err != nil {
		return MetricISDTemplate{}, err
	}*/
	metric.setFilterKey(templateName, scopeVariables)
	metric.setTemplateName(templateName)

	if err = metric.checkMetricTemplateErrors(templateName); err != nil {
		return MetricISDTemplate{}, err
	}
	log.Info("processed template and converting to json", metric)
	return metric, nil
}
