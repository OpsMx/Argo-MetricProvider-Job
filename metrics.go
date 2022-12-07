package main

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Metrics struct {
	MetricType            string   `yaml:"metricType" json:"metricType,omitempty"`
	MetricWeight          *float64 `yaml:"metricWeight" json:"metricWeight,omitempty"`
	NanStrategy           string   `yaml:"nanStrategy" json:"nanStrategy,omitempty"`
	AccountName           string   `yaml:"accountName" json:"accountName,omitempty"`
	RiskDirection         string   `yaml:"riskDirection" json:"riskDirection,omitempty"`
	CustomThresholdHigher int      `yaml:"customThresholdHigherPercentage" json:"customThresholdHigher,omitempty"`
	Name                  string   `yaml:"name" json:"name,omitempty"`
	Criticality           string   `yaml:"criticality" json:"criticality,omitempty"`
	CustomThresholdLower  int      `yaml:"customThresholdLowerPercentage" json:"customThresholdLower,omitempty"`
	Watchlist             bool     `yaml:"watchlist" json:"watchlist"`
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
	FilterKey        string   `yaml:"filterKey" json:"filterKey,omitempty"`
	AccountName      string   `yaml:"accountName" json:"accountName,omitempty"`
	Data             Data     `yaml:"metricTemplateSetup" json:"data"`
	TemplateName     string   `yaml:"templateName" json:"templateName"`
	AdvancedProvider string   `yaml:"advancedProvider" json:"advancedProvider"`
	MetricType       string   `yaml:"metricType" json:"metricType,omitempty"`
	MetricWeight     *float64 `yaml:"metricWeight" json:"metricWeight,omitempty"`
	NanStrategy      string   `yaml:"nanStrategy" json:"nanStrategy,omitempty"`
	Criticality      string   `yaml:"criticality" json:"criticality,omitempty"`
}

func (m *MetricISDTemplate) setMetricType(templateName string) {
	var isMetricTypeSet bool
	if m.MetricType == "" {
		log.Debugf("the metricType field is not defined at the global level for metric template %s", templateName)
	}
	for _, metric := range m.Data.Groups {
		if metric.Metrics[0].MetricType != "" {
			isMetricTypeSet = true
		}
		metric.Metrics[0].MetricType = m.MetricType
	}
	if isMetricTypeSet {
		log.Warnf("the metricType field has been defined at the level of individual metrics for some of the metrics for template %s, metricType field should be defined only at the global level", templateName, m.AccountName)
	}
	m.MetricType = ""
}

func (m *MetricISDTemplate) setMetricWeight(templateName string) {
	//metricWeight
	if m.MetricWeight == nil {
		log.Debugf("the metricWeight field is not defined at the global level for metric template %s, values at the metric level will be taken", templateName)
		return
	}
	for _, metric := range m.Data.Groups {
		if metric.Metrics[0].MetricWeight == nil {
			metric.Metrics[0].MetricWeight = m.MetricWeight
		}
	}
	m.MetricWeight = nil
}

func (m *MetricISDTemplate) setNanStrategy(templateName string) {
	//nanStrategy
	if m.NanStrategy == "" {
		log.Debugf("the nanStrategy field is not defined at the global level for metric template %s, values at the metric level will be taken", templateName)
		return
	}
	for _, metric := range m.Data.Groups {
		if metric.Metrics[0].NanStrategy == "" {
			metric.Metrics[0].NanStrategy = m.NanStrategy
		}
	}
	m.NanStrategy = ""
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
		errMsg := fmt.Sprintf("gitops '%s' template ConfigMap validation error: metric template %s does not have any members defined for the groups field", templateName, templateName)
		return errors.New(errMsg)
	}
	return nil
}

func (m *MetricISDTemplate) setCriticality(templateName string) error {
	//temporary fix -will be handled later on by Java
	//TODO -redo
	for _, metric := range m.Data.Groups {
		if strings.ToLower(metric.Metrics[0].Criticality) == "low" {
			metric.Metrics[0].Criticality = "Normal"
		} else if strings.ToLower(metric.Metrics[0].Criticality) == "medium" {
			metric.Metrics[0].Criticality = "MustHave"
		} else if strings.ToLower(metric.Metrics[0].Criticality) == "high" {
			metric.Metrics[0].Criticality = "Critical"
		} else if metric.Metrics[0].Criticality != "" {
			errMsg := fmt.Sprintf("gitops '%s' template ConfigMap validation error: criticality field can only take values Low/Medium/High", templateName)
			return errors.New(errMsg)
		}
	}
	if m.Criticality != "" {
		if strings.ToLower(m.Criticality) == "low" {
			m.Criticality = "Normal"
		} else if strings.ToLower(m.Criticality) == "medium" {
			m.Criticality = "MustHave"
		} else if strings.ToLower(m.Criticality) == "high" {
			m.Criticality = "Critical"
		} else if m.Criticality != "" {
			errMsg := fmt.Sprintf("gitops '%s' template ConfigMap validation error: criticality field can only take values Low/Medium/High", templateName)
			return errors.New(errMsg)
		}

		for _, metric := range m.Data.Groups {
			if metric.Metrics[0].Criticality == "" {
				metric.Metrics[0].Criticality = m.Criticality
			}
		}
		m.Criticality = ""
	}
	return nil
}

func processYamlMetrics(templateData []byte, templateName string, scopeVariables string) (MetricISDTemplate, error) {
	metric := MetricISDTemplate{}
	err := yaml.Unmarshal(templateData, &metric)
	if err != nil {
		errorMsg := fmt.Sprintf("gitops '%s' template ConfigMap validation error: %v", templateName, err)
		return MetricISDTemplate{}, errors.New(errorMsg)
	}

	metric.setFilterKey(templateName, scopeVariables)
	metric.setTemplateName(templateName)
	metric.setMetricType(templateName)
	metric.setMetricWeight(templateName)
	metric.setNanStrategy(templateName)
	if err = metric.setCriticality(templateName); err != nil {
		return MetricISDTemplate{}, err
	}

	if err = metric.checkMetricTemplateErrors(templateName); err != nil {
		return MetricISDTemplate{}, err
	}
	log.Info("processed template and converting to json", metric)
	return metric, nil
}
