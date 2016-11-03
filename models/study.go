package models

import (
	"fmt"

	"github.com/intervention-engine/riskservice/plugin"
)

// Study represents a single study / patient, containing all of the records making up the study
type Study struct {
	ID                  string
	Records             []Record
}

// AddRecord adds a record to the study, checking to ensure it has the same Study ID
func (s *Study) AddRecord(r Record) error {
	// Make sure the study ID's match, setting the study ID if necessary
	if r.StudyID != "" {
		if s.ID == "" {
			s.ID = r.StudyIDString()
		} else if s.ID != r.StudyIDString() {
			return fmt.Errorf("Record with study ID %s cannot be added to study with ID %s", r.StudyID, s.ID)
		}
	}

	s.Records = append(s.Records, r)

	return nil
}

// ToRiskServiceCalculationResults converts the records to RiskServiceCalculationResults and returns them sorted
// by the AsOf date.  Note that the size of the resulting list may be smaller than the size of the record list since
// some records may represent incomplete risk factors.  The corresponding patientURL must be passed in so the risk pie
// can be assiocated to the patient on the FHIR server.
func (s *Study) ToRiskServiceCalculationResults(patientURL string) []plugin.RiskServiceCalculationResult {
	var results []plugin.RiskServiceCalculationResult
	for i := range s.Records {
		if result, err := s.Records[i].ToRiskServiceCalculationResult(patientURL); err == nil {
			results = append(results, *result)
		}
	}
	plugin.SortResultsByAsOfDate(results)

	return results
}

// StudyMap is a simple map of studies indexed by the study ID, providing a few convenience functions
type StudyMap map[string]*Study

// AddRecord adds a record to the appropriate study in the map.  If no corresponding study is found, adds a new study to
// the map.
func (s StudyMap) AddRecord(r Record) error {
	studyID := r.StudyIDString()
	study, ok := s[studyID]
	if !ok {
		study = new(Study)
		s[studyID] = study
	}
	return study.AddRecord(r)
}

// AddRecords adds a set of records, ensuring each record is associated with its matching study.  If no corresponding
// study is found for a given record, adds a new study to the map.
func (s StudyMap) AddRecords(r []Record) error {
	for i := range r {
		if err := s.AddRecord(r[i]); err != nil {
			return err
		}
	}
	return nil
}
