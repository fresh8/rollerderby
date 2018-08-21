## Roller Derby

Roller derby manages rolling replacements of services that use Instance metadata
for the target revision of an application.

## Authentication

There are two mechanisms you can use for authentication:

 1. user credentials (e.g. `google auth login --project $PROJECT_ID`).
 2. account key credentials (e.g. service account JSON file).

NOTE: for each form of authentication Roller Derby's actions are restricted to
the roles IAM configuration.

### 1. Using User Credentials

To use user credentials carry out the following steps:

 1. Download and install [Google Cloud SDK](https://cloud.google.com/sdk/).
 2. Login to each environment with `google auth login --project $PROJECT_ID`.

 ### 2. Using Account Keys

To use account keys carry out the following steps:

 1. Create and download the account key under the IAM service accounts panel.
 2. Move the key to a secure path setting permissions as appropriate.
 3. Export an environment variable pointing to your JSON key
    (e.g. `export GOOGLE_APPLICATION_CREDENTIALS="gcp.json"`).


## Help

Command line help output:

```
Usage of rollerderby:
  -compare string
    	compare this projects meta to the default projects
  -groups
    	list compute instance groups and exit
  -key string
    	metadata key to update
  -meta
    	list projects common metadata key values
  -project string
    	Google project ID, can be set with this flag or GOOGLE_PROJECT_ID environment variable
  -target string
    	target instance group to replace
  -value string
    	metadata value to set
  -version
    	output version and exit
```

### Compare Metadata

Compare metadata prints a table of all the metadata values highlighting where
values are the same.

```
$ rollerderby -project=project-a -compare=project-b
project: project-a
version: d3809c3dc4a4c614965ba8761c883dbf5a583352
source: git@github.com:ConnectedVentures/rollerderby.git
go: go1.10.3
auth: <gcloud auth>
key                                           | equal | project-a                 | project-b
=============================================================================================================
env                                           | false | production                | staging
```

### List Metadata

List metadata prints a table of the values for the target project.

```
$ rollerderby -meta -project=project-a
project: project-a
version: d3809c3dc4a4c614965ba8761c883dbf5a583352
source: git@github.com:ConnectedVentures/rollerderby.git
go: go1.10.3
auth: <gcloud auth>
key                                           | project-a
==============================================================================
env                                           | staging
```

### Target

Target does a rolling upgrade first upserting the key with the specified value
and then replacing the instances in the related instance group.

```
$ rollerderby -project=project-a -target=app-group -key=app_version -value=1.0.1
2018/08/16 20:12:19 main.go:67: project: project-a
2018/08/16 20:12:19 main.go:68: version: d3809c3dc4a4c614965ba8761c883dbf5a583352
2018/08/16 20:12:19 main.go:69: source: git@github.com:ConnectedVentures/rollerderby.git
2018/08/16 20:12:19 main.go:70: go: go1.10.3
2018/08/16 20:12:19 main.go:74: auth: <gcloud auth>
2018/08/16 20:12:20 gcp_compute.go:223: wrote current metadata to project-a-1534450340.json
2018/08/16 20:12:20 gcp_compute.go:261: fingerprint: jy9AERL4jzY=
2018/08/16 20:12:20 gcp_compute.go:264: app_version: 1.0.0 -> 1.0.1
2018/08/16 20:12:22 gcp_compute.go:55: replacing group app-group instances [zones/europe-west1-d/instances/app-group-7bv1]
```

