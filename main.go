package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
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
	log.Println("auth:", authPath)
	log.Println("project:", projectID)

	configErrors := validateConfig(projectID, authPath, key, newValue)
	if configErrors != nil {
		log.Fatalf("%s\n", configErrors)
	}

	ctx := context.Background()

	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		log.Fatal(err)
	}

	service, err := compute.New(client)
	if err != nil {
		log.Fatal(err)
	}

	// retrieve current values
	projectService := compute.NewProjectsService(service)
	project, err := projectService.Get(projectID).Do()
	if err != nil {
		log.Fatal(err)
	}

	filename := fmt.Sprintf("%v-%d.json", projectID, time.Now().Unix())
	w, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}

	enc := json.NewEncoder(w)
	err = enc.Encode(&project.CommonInstanceMetadata)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("wrote current metadata to", filename)

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
		newItem := compute.MetadataItems{
			Key:   key,
			Value: &newValue,
		}
		project.CommonInstanceMetadata.Items = append(project.CommonInstanceMetadata.Items, &newItem)
		log.Printf("%s: <EMPTY> -> %s\n", key, newValue)
	} else {
		project.CommonInstanceMetadata.Items[keyIndex].Value = &newValue
	}

	op, err := projectService.SetCommonInstanceMetadata(projectID, project.CommonInstanceMetadata).Do()
	if err != nil {
		log.Fatal(err)
	}

	if op.Error != nil {
		log.Fatalf("%+v\n", op.Error.Errors)
	}

	// replace existing servers in service group 1 at a time

	// poll until done
}

func validateConfig(projectID, authPath, key, value string) Errors {
	var errors Errors
	if projectID == "" {
		errors = append(errors, fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	/*
	if authPath == "" {
		errors = append(errors, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS cannot be blank"))
	}
	*/

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
