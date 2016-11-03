Multi-Factor Risk Service [![Build Status](https://travis-ci.org/intervention-engine/multifactorriskservice.svg?branch=master)](https://travis-ci.org/intervention-engine/multifactorriskservice)
=================================================================================================================================================================================================

The *multifactorriskservice* project provides a prototype risk service server for the [Intervention Engine](https://github.com/intervention-engine/ie) project. The *multifactorriskservice* server interfaces with a [REDCap](http://projectredcap.org/) database to import recorded risk scores for patients, based on a multi-factor risk model.  The *multifactorriskservice* server also provides risk component data in a format that allows the Intervention Engine [frontend](https://github.com/intervention-engine/frontend) to properly draw the "risk pies".

The integration with REDCap supports our current use case, but users outside our organization don't likely have access to a REDCap server or the specific database referenced by the multi-factor risk service.  For this reason, the *multifactorriskservice* provides a *mock* implementation for generating synthetic risk scores to allow testing and development without a REDCap server.  *The mock implementation must ONLY be used for development with synthetic patients.  It should never be used with production (real) data!*

Building and Running multifactorriskservice Locally
---------------------------------------------------

Intervention Engine is a stack of tools and technologies. For information on installing and running the full stack, please see [Building and Running the Intervention Engine Stack in a Development Environment](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md).

For information related specifically to building and running the code in this repository (*multifactorriskservice*), please refer to the following sections in the above guide. Note that the risk service is useless without the Intervention Engine server, so it is listed as a prerequisite.

-	(Prerequisite) [Install Git](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#install-git)
-	(Prerequisite) [Install Go](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#install-go)
-	(Prerequisite) [Install MongoDB](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#install-mongodb)
-	(Prerequisite) [Run MongoDB](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#run-mongodb)
-	(Prerequisite) [Clone ie Repository](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#clone-ie-repository)
-	(Prerequisite) [Build and Run Intervention Engine Server](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#build-and-run-intervention-engine-server)
-	(Optional) [Create Intervention Engine User](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#create-intervention-engine-user)
-	(Optional) [Generate and Upload Synthetic Patient Data](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#generate-and-upload-synthetic-patient-data)
-	[Clone multifactorriskservice Repository](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#clone-multifactorriskservice-repository)
-	[Build and Run Multi-Factor Risk Service Server](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#build-and-run-multi-factor-risk-service-server)

Building and Running the multifactorriskservice MOCK Service
------------------------------------------------------------

The mock service will generate synthetic multi-factor risk assessments for each patient in the FHIR database.  Since these assessments are fake, it is very important that the mock service *never* be used with real patient data.  If you are sure your FHIR database only contains synthetic data, you may proceed with the following instructions to build and run the mock multi-factor risk service.

Before you can run the MOCK Multi-Factor Risk Service server, you must install its dependencies via `go get` and build the `mock` executable:

```
$ cd $GOPATH/src/github.com/intervention-engine/multifactorriskservice/mock
$ go get
$ go build
```

The above commands do not need to be run again unless you make (or download) changes to the *multifactorriskservice* source code.

The `mock` executable requires a `confirm-mock` argument, a `-fhir` argument to indicate the URL of the FHIR API server, and an optional `-gen` argument to indicate that mock assessments should be generated immediately.  Note that the `-confirm-mock` argument exists as a safety measure to ensure that the user really intends to generate fake data.

```
$ ./mock -confirm-mock -fhir http://localhost:3001 -gen
```

If the `-gen` flag is not passed, mock assessments will not be generated and the service will simply serve the existing mock assessment data.

To trigger a generation (or refresh) of the mock assessments at any time, issue an HTTP POST to [http://localhost:9000/refresh](http://localhost:9000/refresh).

```
curl -X POST http://localhost:9000/refresh
```

The mock server accepts connections on port 9000 by default.

License
-------

Copyright 2016 The MITRE Corporation

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
