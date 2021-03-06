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
	err := exec()
	if err != nil {
		log.Fatal(err)
	}
}

func exec() error {
	var key string
	var newValue string
	var projectID string
	var otherProjectID string
	var groupName string
	var authPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var zoneName string
	var minReadySec int64
	var listMeta bool
	var listVersion bool
	var listGroups bool

	flag.StringVar(&projectID, "project", os.Getenv("GOOGLE_PROJECT_ID"), "Google project ID, can be set with this flag or GOOGLE_PROJECT_ID environment variable")
	flag.StringVar(&key, "key", "", "metadata key to update")
	flag.StringVar(&newValue, "value", "", "metadata value to set")
	flag.StringVar(&otherProjectID, "compare", "", "compare this projects meta to the default projects")
	flag.StringVar(&groupName, "target", "", "target instance group to replace")
	flag.StringVar(&zoneName, "zone", os.Getenv("GOOGLE_ZONE"), "target instance group to replace")
	flag.Int64Var(&minReadySec, "ready", 90, "minimum number of seconds to wait before assuming the service is ready")
	flag.BoolVar(&listMeta, "meta", false, "list projects common metadata key values")
	flag.BoolVar(&listVersion, "version", false, "output version and exit")
	flag.BoolVar(&listGroups, "groups", false, "list compute instance groups and exit")
	flag.Parse()

	if zoneName == "" {
		zoneName = "europe-west1-d"
	}

	log.SetFlags(log.LUTC | log.Lshortfile | log.LstdFlags)
	// for user interactive output remove extended log info
	if listMeta || otherProjectID != "" || listVersion || listGroups {
		log.SetFlags(0)
	}

	// output target environment details
	printConfig(authPath, projectID, Version, Source)

	if listVersion {
		return nil
	} else if otherProjectID != "" {
		keys, err := compute.CompareProjects(projectID, otherProjectID)
		if err != nil {
			return err
		}
		compute.PrintKeys(keys, projectID, otherProjectID)

		return nil
	} else if listMeta {
		return compute.ListKeys(projectID)
	} else if listGroups {
		return compute.ListInstanceGroups(projectID)
	} else if projectID != "" && key != "" && newValue != "" && groupName != "" {
		err := compute.UpdateKey(projectID, key, newValue)
		if err != nil {
			return err
		}

		// TODO (NF 2018-08-15): replace with zone look-up for instance group.
		return compute.RollingReplace(projectID, zoneName, groupName, minReadySec)
	} else if projectID != "" && key != "" && newValue != "" {
		return compute.UpdateKey(projectID, key, newValue)
	}

	return nil
}

func printConfig(authPath, projectID, version, source string) {
	log.Println("project:", projectID)
	log.Println("version:", version)
	log.Println("source:", source)
	log.Println("go:", runtime.Version())
	if authPath != "" {
		log.Println("auth:", authPath)
		return
	}

	log.Println("auth: <gcloud auth>")
}
