package main

import (
	"flag"
	"log"
	"os"
	"runtime"

	"github.com/fresh8/rollerderby/compute"
)

// Version is the Git SHA for this application specified at compile time.
var Version string

// Source is the Git origin for this application specified at compile time.
var Source string

func main() {
	log.SetOutput(os.Stdout)

	var key string
	var newValue string
	var projectID string
	var otherProjectID string
	var groupName string
	var authPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var listMeta bool
	var listVersion bool
	var listGroups bool

	flag.StringVar(&projectID, "project", os.Getenv("GOOGLE_PROJECT_ID"), "Google project ID, can be set with this flag or GOOGLE_PROJECT_ID environment variable")
	flag.StringVar(&key, "key", "", "metadata key to update")
	flag.StringVar(&newValue, "value", "", "metadata value to set")
	flag.StringVar(&otherProjectID, "compare", "", "compare this projects meta to the default projects")
	flag.StringVar(&groupName, "target", "", "target instance group to replace")
	flag.BoolVar(&listMeta, "meta", false, "list projects common metadata key values")
	flag.BoolVar(&listVersion, "version", false, "output version and exit")
	flag.BoolVar(&listGroups, "groups", false, "list compute instance groups and exit")
	flag.Parse()

	if listMeta || otherProjectID != "" || listVersion || listGroups {
		log.SetFlags(0)
	} else {
		log.SetFlags(log.LUTC | log.Lshortfile | log.LstdFlags)
	}

	// output target environment details
	printConfig(authPath, projectID)

	if listVersion {
		return
	}

	if otherProjectID != "" {
		compute.CompareProjects(projectID, otherProjectID)
		return
	} else if listMeta {
		compute.ListKeys(projectID)
		return
	} else if listGroups {
		compute.ListInstanceGroups(projectID)
		return
	}

	compute.UpdateKey(projectID, key, newValue)

	// TODO (NF 2018-08-15): replace with zone look-up for instance group.
	if groupName != "" {
		compute.RollingReplace(projectID, "europe-west1-d", groupName)
	}
}

func printConfig(authPath, projectID string) {
	log.Println("project:", projectID)
	log.Println("version:", Version)
	log.Println("source:", Source)
	log.Println("go:", runtime.Version())
	if authPath != "" {
		log.Println("auth:", authPath)
	} else {
		log.Println("auth: <gcloud auth>")
	}
}
