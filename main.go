package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron"

	"gopkg.in/mgo.v2"

	"github.com/intervention-engine/multifactorriskservice/server"
)

func main() {
	httpFlag := flag.String("http", "", "HTTP service address to listen on (env: HTTP_HOST_AND_PORT, default: \":9000\")")
	mongoFlag := flag.String("mongo", "", "MongoDB address (env: MONGO_URL, default: \"mongodb://localhost:27017\")")
	fhirFlag := flag.String("fhir", "", "FHIR API address (env: FHIR_URL, default: \"http://localhost:3001\")")
	redcapFlag := flag.String("redcap", "", "REDCap API address (required, env: REDCAP_URL, example: \"http://redcapsrv:80\")")
	tokenFlag := flag.String("token", "", "REDCap API token (required, env: REDCAP_TOKEN, example: \"F65EBA22DCB728FEC5ADFAD42378CA40\")")
	cronFlag := flag.String("cron", "", "Cron expression indicating when risk assessments should be automatically refreshed (env: REDCAP_CRON, default: \"0 0 22 * * *\")")
	flag.Parse()

	// Prefer http arg, falling back to env, falling back to default
	httpa := getConfigValue(httpFlag, "HTTP_HOST_AND_PORT", ":9000")

	// Prefer mongo arg, falling back to env, falling back to default
	mongo := getConfigValue(mongoFlag, "MONGO_URL", "mongodb://localhost:27017")
	if strings.HasPrefix(mongo, ":") {
		mongo = "mongodb://localhost" + mongo
	}

	// Prefer fhir arg, falling back to env, falling back to default
	fhir := getConfigValue(fhirFlag, "FHIR_URL", "http://localhost:3001")
	if strings.HasPrefix(fhir, ":") {
		fhir = "http://localhost" + fhir
	}

	redcap := getRequiredConfigValue(redcapFlag, "REDCAP_URL", "REDCap URL")
	token := getRequiredConfigValue(tokenFlag, "REDCAP_TOKEN", "REDCap API Token")
	cronSpec := getConfigValue(cronFlag, "REDCAP_CRON", "0 0 22 * * *")

	session, err := mgo.Dial(mongo)
	if err != nil {
		panic("Can't connect to the database")
	}
	defer session.Close()
	db := session.DB("riskservice")
	pieCollection := db.C("pies")

	// Get own endpoint address, falling back to discovery if needed
	endpoint := httpa
	if strings.HasPrefix(endpoint, ":") {
		endpoint = discoverSelf() + endpoint
	}
	basisPieURL := "http://" + endpoint + "/pies"

	// Setup the cron job and start the scheduler
	c := cron.New()
	err = server.ScheduleRefreshRiskAssessmentsCron(c, cronSpec, fhir, redcap, token, pieCollection, basisPieURL)
	if err != nil {
		panic("Can't setup cron job for refreshing risk assessments.  Specified spec: " + cronSpec)
	}
	c.Start()
	defer c.Stop()

	// Create the gin engine, register the routes, and run!
	e := gin.Default()
	server.RegisterRoutes(e, fhir, redcap, token, pieCollection, basisPieURL)
	e.Run(httpa)
}

func getConfigValue(parsedFlag *string, envVar string, defaultVal string) string {
	val := *parsedFlag
	if val == "" {
		val = os.Getenv(envVar)
		if val == "" {
			val = defaultVal
		}
	}
	return val
}

func getRequiredConfigValue(parsedFlag *string, envVar string, name string) string {
	val := getConfigValue(parsedFlag, envVar, "")
	if val == "" {
		fmt.Fprintf(os.Stderr, "%s must be passed in as an argument or environment variable.\n", name)
		flag.PrintDefaults()
		os.Exit(1)
	}
	return val
}

func discoverSelf() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println("Unable to determine IP address.  Defaulting to localhost.")
		return "localhost"
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	log.Println("Unable to determine IP address.  Defaulting to localhost.")
	return "localhost"
}
