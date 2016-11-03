package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"gopkg.in/mgo.v2"

	"sync"

	fhir "github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/multifactorriskservice/models"
	"github.com/intervention-engine/riskservice/service"
)

var m sync.Mutex

// RefreshRiskAssessments pulls the risk assessment data from REDCap and posts it to the FHIR server, replacing older
// risk assessments and storing pie representations.
func RefreshRiskAssessments(fhirEndpoint string, redcapEndpoint string, redcapToken string, pieCollection *mgo.Collection, basisPieURL string) ([]Result, error) {
	m.Lock()
	defer m.Unlock()
	studies, err := GetREDCapData(redcapEndpoint, redcapToken)
	if err != nil {
		return nil, err
	}
	return PostRiskAssessments(fhirEndpoint, studies, pieCollection, basisPieURL), nil
}

// GetREDCapData queries REDCap at the specified endpoint with the specifed token, returning a StudyMap containing
// the resulting data.
func GetREDCapData(endpoint string, token string) (models.StudyMap, error) {
	form := url.Values{}
	form.Set("token", token)
	form.Set("content", "record")
	form.Set("format", "json")
	form.Set("returnFormat", "json")
	form.Set("type", "flat")
	form.Set("fields", "study_id, redcap_event_name, rf_date, rf_cmc_risk_cat, rf_func_risk_cat, rf_sb_risk_cat, rf_util_risk_cat, rf_risk_predicted")

	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	res, err := http.DefaultClient.PostForm(endpoint, form)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var records []models.Record
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&records); err != nil {
		return nil, err
	}

	m := make(models.StudyMap)
	if err := m.AddRecords(records); err != nil {
		return nil, err
	}

	return m, nil
}

// PostRiskAssessments posts the risk assessments from the studies to the FHIR server and also stores the risk pies
// to the local Mongo database
func PostRiskAssessments(fhirEndpoint string, studies models.StudyMap, pieCollection *mgo.Collection, basisPieURL string) []Result {
	results := make([]Result, 0, len(studies))
	for _, study := range studies {
		result := Result{
			StudyID: study.ID,
		}
		// Query the FHIR server to find the patient ID by the Study ID (often the MRN)
		r, err := http.NewRequest("GET", fhirEndpoint+"/Patient?identifier="+study.ID, nil)
		if err != nil {
			result.Error = fmt.Errorf("Couldn't create HTTP request for querying patient with Study ID: %s.  Error: %s", study.ID, err.Error())
			results = append(results, result)
			continue
		}
		r.Header.Set("Accept", "application/json")
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			result.Error = fmt.Errorf("Couldn't query FHIR server for patient with Study ID: %s.  Error: %s", study.ID, err.Error())
			results = append(results, result)
			continue
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			result.Error = fmt.Errorf("Received HTTP %d %s from FHIR server when querying patient with Study ID: %s.", res.StatusCode, res.Status, study.ID)
			results = append(results, result)
			continue
		}
		var patients fhir.Bundle
		decoder := json.NewDecoder(res.Body)
		if err := decoder.Decode(&patients); err != nil {
			result.Error = fmt.Errorf("Couldn't properly decode results from patient query with Study ID: %s.  Error: %s", study.ID, err.Error())
			results = append(results, result)
			continue
		}
		if len(patients.Entry) == 0 {
			result.Error = fmt.Errorf("Couldn't find patient with Study ID %s", study.ID)
			results = append(results, result)
			continue
		} else if len(patients.Entry) > 1 {
			result.Error = fmt.Errorf("Found too many patients (%d) with Study ID %s", len(patients.Entry), study.ID)
			results = append(results, result)
			continue
		}
		patientID := patients.Entry[0].Resource.(*fhir.Patient).Id
		result.FHIRPatientID = patientID

		// Get the risk assessments from the records, post to FHIR server, and update pies in Mongo
		calcResults := study.ToRiskServiceCalculationResults(fhirEndpoint + "/Patient/" + patientID)
		err = service.UpdateRiskAssessmentsAndPies(fhirEndpoint, patientID, calcResults, pieCollection, basisPieURL, REDCapRiskServiceConfig)
		if err != nil {
			result.Error = err
		} else {
			result.RiskAssessmentCount = len(calcResults)
		}
		results = append(results, result)
	}

	return results
}

// Result represents the result (successful or not) of posting REDCap risk assessments to a FHIR server
type Result struct {
	StudyID             string
	FHIRPatientID       string
	RiskAssessmentCount int
	Error               error
}

// MarshalJSON handles the marshalling of the errors since Go doesn't
func (r *Result) MarshalJSON() ([]byte, error) {
	var errString string
	if r.Error != nil {
		errString = r.Error.Error()
	}
	return json.Marshal(&struct {
		StudyID             string `json:"studyID,omitempty"`
		FHIRPatientID       string `json:"fhirPatientID,omitempty"`
		RiskAssessmentCount int    `json:"riskAssessmentCount"`
		Error               string `json:"error,omitempty"`
	}{
		StudyID:             r.StudyID,
		FHIRPatientID:       r.FHIRPatientID,
		RiskAssessmentCount: r.RiskAssessmentCount,
		Error:               errString,
	})
}

// LogResultSummary prints out a log of the result summary (# patients, # errors, # assessments)
func LogResultSummary(results []Result) {
	// Log out some information
	var numErrors, numAssessments int
	for _, result := range results {
		if result.Error != nil {
			numErrors++
		}
		numAssessments += result.RiskAssessmentCount
	}
	log.Printf("Refreshed risk assessments for %d patients: %d errors, %d risk assessments.",
		len(results), numErrors, numAssessments)
}
