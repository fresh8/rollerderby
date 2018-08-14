package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var Version string
var Source string

func main() {
	log.SetFlags(log.LUTC | log.Lshortfile | log.LstdFlags)

	var key string
	var newValue string
	var projectID string
	var authPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	flag.StringVar(&projectID, "project", os.Getenv("GOOGLE_PROJECT_ID"), "Google project ID, can be set with this flag or GOOGLE_PROJECT_ID environment variable")
	flag.StringVar(&key, "key", "", "metadata key to update")
	flag.StringVar(&newValue, "value", "", "metadata value to set")
	flag.Parse()

	// output target environment details
	log.Println("version:", Version)
	log.Println("source:", Source)
	if authPath != "" {
		log.Println("auth:", authPath)
	} else {
		log.Println("auth: <gcloud auth>")
	}

	log.Println("project:", projectID)

	configErrors := validateConfig(projectID, authPath, key, newValue)
	if configErrors != nil {
		log.Fatalf("%s\n", configErrors)
	}

	computeService := computeClient()

	projectService := compute.NewProjectsService(computeService)

	// TODO (NF 2018-08-10): Retry loop when a fingerprint doesn't match (aka a concurrent write).
	// retrieve current values
	project := getProject(projectService, projectID)

	filename := fmt.Sprintf("%v-%d.json", project.Name, time.Now().Unix())
	w, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}

	writeMeta(w, project.CommonInstanceMetadata)
	log.Println("wrote current metadata to", filename)

	updateCommonKey(project, key, newValue, projectService)

	// replace existing servers in service group 1 at a time

	// poll until done
}

func computeClient() *compute.Service {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		log.Fatal(err)
	}
	service, err := compute.New(client)
	if err != nil {
		log.Fatal(err)
	}

	return service
}

func getProject(projectService *compute.ProjectsService, projectID string) *compute.Project {
	project, err := projectService.Get(projectID).Do()
	if err != nil {
		log.Fatal(err)
	}
	return project
}

func writeMeta(w io.WriteCloser, meta *compute.Metadata) {
	enc := json.NewEncoder(w)
	err := enc.Encode(meta)
	if err != nil {
		log.Fatal(err)
	}
}

func updateCommonKey(project *compute.Project, key string, newValue string, projectService *compute.ProjectsService) {
	keyIndex := -1
	log.Println("fingerprint:", project.CommonInstanceMetadata.Fingerprint)
	for i, meta := range project.CommonInstanceMetadata.Items {
		if meta.Key == key {
			log.Printf("%s: %s -> %s\n", meta.Key, *meta.Value, newValue)
			keyIndex = i
		}
	}

	// update new values in metadata
	if keyIndex == -1 {
		keyIndex = len(project.CommonInstanceMetadata.Items)
		newItem := compute.MetadataItems{
			Key: key,
		}
		project.CommonInstanceMetadata.Items = append(project.CommonInstanceMetadata.Items, &newItem)
		log.Printf("%s: <EMPTY> -> %s\n", key, newValue)
	}

	project.CommonInstanceMetadata.Items[keyIndex].Value = &newValue

	op, err := projectService.SetCommonInstanceMetadata(project.Name, project.CommonInstanceMetadata).Do()
	if err != nil {
		log.Fatal(err)
	}
	if op.Error != nil {
		log.Fatalf("%+v\n", op.Error.Errors)
	}
}

func validateConfig(projectID, authPath, key, value string) Errors {
	var errors Errors
	if projectID == "" {
		errors = append(errors, fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	if key == "" {
		errors = append(errors, fmt.Errorf("-key cannot be blank"))
	}

	if key == "" {
		errors = append(errors, fmt.Errorf("-value cannot be blank"))
	}

	return errors
}

type Errors []error

func (errs Errors) String() string {
	var s []string
	for _, err := range errs {
		s = append(s, err.Error())
	}
	return strings.Join(s, "; ")
}
