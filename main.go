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
	"sort"
	"strings"
	"time"
)

var Version string
var Source string

func main() {
	log.SetOutput(os.Stdout)

	var key string
	var newValue string
	var projectID string
	var otherProjectID string
	var authPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var listMeta bool

	flag.StringVar(&projectID, "project", os.Getenv("GOOGLE_PROJECT_ID"), "Google project ID, can be set with this flag or GOOGLE_PROJECT_ID environment variable")
	flag.StringVar(&key, "key", "", "metadata key to update")
	flag.StringVar(&newValue, "value", "", "metadata value to set")
	flag.StringVar(&otherProjectID, "compare", "", "compare this projects meta to the default projects")
	flag.BoolVar(&listMeta, "list", false, "list project common meta key values")
	flag.Parse()

	if listMeta || otherProjectID != "" {
		log.SetFlags(0)
	} else {
		log.SetFlags(log.LUTC | log.Lshortfile | log.LstdFlags)
	}

	// output target environment details
	log.Println("version:", Version)
	log.Println("source:", Source)
	if authPath != "" {
		log.Println("auth:", authPath)
	} else {
		log.Println("auth: <gcloud auth>")
	}

	log.Println("project:", projectID)

	if otherProjectID != "" {
		compareProjects(projectID, otherProjectID)
		return
	} else if listMeta {
		listKeys(projectID)
		return
	}

	updateKey(projectID, key, newValue)
}

type CompareMeta struct {
	A string
	B string
}

func compareProjects(projectA, projectB string) {
	if projectA == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	computeService := computeClient()
	projectService := compute.NewProjectsService(computeService)

	aInfo := getProject(projectService, projectA)
	bInfo := getProject(projectService, projectB)
	keys := make(map[string]CompareMeta)
	for _, item := range aInfo.CommonInstanceMetadata.Items {
		// TODO should probably check for dups here rather than assume anything.
		keys[item.Key] = CompareMeta{
			A: *item.Value,
		}
	}

	for _, item := range bInfo.CommonInstanceMetadata.Items {
		v, ok := keys[item.Key]
		if !ok {
			v = CompareMeta{}
		}

		v.B = *item.Value
		keys[item.Key] = v
	}

	log.Printf("%-45.45s | %-5.5s | %-25.25s | %-25.25s\n", "key", "equal", projectA, projectB)
	log.Printf("%s\n", strings.Repeat("=", 45 + 5 + 2*25 + 3*3))
	for k := range keys {
		log.Printf("%-45.45s | %-5.5t | %-25.25s | %-25.25s\n", k, keys[k].A == keys[k].B, keys[k].A, keys[k].B)
	}
}

func listKeys(projectID string) {
	if projectID == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	computeService := computeClient()
	projectService := compute.NewProjectsService(computeService)

	project := getProject(projectService, projectID)
	sort.Sort(ItemsByKey(project.CommonInstanceMetadata.Items))
	for _, meta := range project.CommonInstanceMetadata.Items {
		log.Printf("%s = %.40s\n", meta.Key, *meta.Value)
	}
}

type ItemsByKey []*compute.MetadataItems

func (m ItemsByKey) Len() int           { return len(m) }
func (m ItemsByKey) Less(i, j int) bool { return m[i].Key < m[j].Key }
func (m ItemsByKey) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

func updateKey(projectID string, key string, newValue string) {
	configErrors := validateUpdateParms(projectID, key, newValue)
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

func validateUpdateParms(projectID, key, value string) Errors {
	var errors Errors
	if projectID == "" {
		errors = append(errors, fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	if key == "" {
		errors = append(errors, fmt.Errorf("-key cannot be blank"))
	}

	if value == "" {
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
