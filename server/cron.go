package server

import (
	"log"

	"github.com/intervention-engine/multifactorriskservice/client"
	"github.com/robfig/cron"
	"gopkg.in/mgo.v2"
)

// ScheduleRefreshRiskAssessmentsCron schedules a cron job for refreshing the risk assessments
func ScheduleRefreshRiskAssessmentsCron(c *cron.Cron, spec string, fhirEndpoint, redcapEndpoint, redcapToken string, pieCollection *mgo.Collection, basisPieURL string) error {
	return c.AddFunc(spec, func() {
		results, err := client.RefreshRiskAssessments(fhirEndpoint, redcapEndpoint, redcapToken, pieCollection, basisPieURL)
		if err != nil {
			log.Println("Error refreshing risk assessments", err)
		} else {
			client.LogResultSummary(results)
		}
	})
}
