package client

import (
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/riskservice/plugin"
)

var REDCapRiskServiceConfig = plugin.RiskServicePluginConfig{
	Name: "Multi-Factor Risk Service",
	Method: models.CodeableConcept{
		Coding: []models.Coding{{System: "http://interventionengine.org/risk-assessments", Code: "MultiFactor"}},
		Text:   "Multi-Factor",
	},
	PredictedOutcome: models.CodeableConcept{Text: "Catastrophic Health Event"},
	DefaultPieSlices: []plugin.Slice{
		{Name: "Clinical Risk", Weight: 25, MaxValue: 4},
		{Name: "Functional and Environmental Risk", Weight: 25, MaxValue: 4},
		{Name: "Psychosocial and Mental Health Risk", Weight: 25, MaxValue: 4},
		{Name: "Utilization Risk", Weight: 25, MaxValue: 4},
	},
	RequiredResourceTypes: []string{},
}
