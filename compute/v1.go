package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fresh8/rollerderby/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// CompareMeta is a struct to contain the meta values between two projects for
// the same key.
type CompareMeta struct {
	A string
	B string
}

// ReplaceAllInstances replaces all instances of a instance group.
func ReplaceAllInstances(projectID, groupName string) {
	if projectID == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID/-project cannot be blank"))
	}

	if groupName == "" {
		log.Fatal(fmt.Errorf("instance group name cannot be blank"))
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		log.Fatal(err)
	}

	igms := compute.NewInstanceGroupManagersService(computeService)

	list, err := aggregatedList(igms, projectID)
	if err != nil {
		log.Fatal(err)
	}

	for k, item := range list.Items {
		if len(item.InstanceGroupManagers) > 0 {
			log.Println(k) // print zone
		}
	}
}

// ReplaceInstances replaces instances of a group in the zone specified.
func ReplaceInstances(projectID, zone, groupName string) {
	if projectID == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	if zone == "" {
		log.Fatal(fmt.Errorf("zone cannot be blank"))
	}

	if groupName == "" {
		log.Fatal(fmt.Errorf("instance group name cannot be blank"))
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		log.Fatal(err)
	}

	igms := compute.NewInstanceGroupManagersService(computeService)

	instances, err := ListManagedInstances(igms, projectID, zone, groupName)
	if err != nil {
		log.Fatal(err)
	}

	op, err := igms.RecreateInstances(projectID, zone, groupName, &compute.InstanceGroupManagersRecreateInstancesRequest{Instances: instances}).Do()
	if err != nil {
		log.Fatal(err)
	}

	if op.Error != nil {
		for _, e := range op.Error.Errors {
			log.Println(e.Code, e.Message, e.Location)
		}
		log.Fatalf("got op.Error != nil, want nil")
	}

	log.Printf("replacing group %v instances %v", groupName, instances)
}

// ListInstanceGroups prints all instance groups for the given projectID.
func ListInstanceGroups(projectID string) {
	if projectID == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		log.Fatal(err)
	}

	igms := compute.NewInstanceGroupManagersService(computeService)

	list, err := aggregatedList(igms, projectID)
	if err != nil {
		log.Fatal(err)
	}

	for k, item := range list.Items {
		if len(item.InstanceGroupManagers) > 0 {
			log.Println(k) // print zone
		}

		for _, v := range item.InstanceGroupManagers {
			log.Println("    ", v.Name, v.InstanceGroup) // instance group
			a := strings.Split(v.Zone, "/")

			instances, err := ListManagedInstances(igms, projectID, a[len(a)-1], v.Name)
			if err != nil {
				log.Fatal(err)
			}

			for _, instance := range instances {
				log.Println("        ", instance) // instances
			}
		}
	}
}

func aggregatedList(igms *compute.InstanceGroupManagersService, projectID string) (*compute.InstanceGroupManagerAggregatedList, error) {
	return igms.AggregatedList(projectID).Do()
}

// ListManagedInstances lists the managed instances for the given projectID, zone, and groupName.
func ListManagedInstances(igms *compute.InstanceGroupManagersService, projectID, zone, groupName string) ([]string, error) {
	list, err := igms.ListManagedInstances(projectID, zone, groupName).Do()
	if err != nil {
		return nil, err
	}

	var result []string
	for _, instance := range list.ManagedInstances {
		if instance == nil {
			continue
		}

		a := strings.Split(instance.Instance, "/")

		result = append(result, strings.Join(a[len(a)-4:], "/"))
	}

	return result, nil
}

// CompareProjects prints a comparison table between projectA and projectB.
func CompareProjects(projectA, projectB string) {
	if projectA == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		log.Fatal(err)
	}

	projectService := compute.NewProjectsService(computeService)

	aInfo, err := getProject(projectService, projectA)
	if err != nil {
		log.Fatal(err)
	}

	bInfo, err := getProject(projectService, projectB)
	if err != nil {
		log.Fatal(err)
	}

	keys := make(map[string]CompareMeta)
	for _, item := range aInfo.CommonInstanceMetadata.Items {
		// TODO should probably check for dups here rather than assume anything.
		_, ok := keys[item.Key]
		if ok {
			log.Printf("WARNING: duplicate key seen in first projects metadata %v\n", item.Key)
		}
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
	log.Printf("%s\n", strings.Repeat("=", 45+5+2*25+3*3))
	for k := range keys {
		log.Printf("%-45.45s | %5t | %-25.25s | %-25.25s\n", k, keys[k].A == keys[k].B, keys[k].A, keys[k].B)
	}
}

// ListKeys prints a list of all keys associated with a projectID.
func ListKeys(projectID string) {
	if projectID == "" {
		log.Fatal(fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank"))
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		log.Fatal(err)
	}

	projectService := compute.NewProjectsService(computeService)

	project, err := getProject(projectService, projectID)
	if err != nil {
		log.Fatal(err)
	}
	sort.Sort(ItemsByKey(project.CommonInstanceMetadata.Items))
	log.Printf("%-45.45s | %-30.30s\n", "key", projectID)
	log.Printf("%s\n", strings.Repeat("=", 45+30+1*3))
	for _, meta := range project.CommonInstanceMetadata.Items {
		log.Printf("%-45.45s | %-30.30s\n", meta.Key, *meta.Value)
	}
}

// ItemsByKey is a sortable interface for metadata items.
type ItemsByKey []*compute.MetadataItems

func (m ItemsByKey) Len() int           { return len(m) }
func (m ItemsByKey) Less(i, j int) bool { return m[i].Key < m[j].Key }
func (m ItemsByKey) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// UpdateKey updates the projects common metadata key with newValue.
func UpdateKey(projectID string, key string, newValue string) {
	configErrors := validateUpdateParms(projectID, key, newValue)
	if configErrors != nil {
		log.Fatalf("%s\n", configErrors)
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		log.Fatal(err)
	}

	projectService := compute.NewProjectsService(computeService)
	// TODO (NF 2018-08-10): Retry loop when a fingerprint doesn't match (aka a concurrent write).
	// retrieve current values
	project, err := getProject(projectService, projectID)
	if err != nil {
		log.Fatal(err)
	}

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

func v1ComputeClient() (*compute.Service, error) {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, err
	}
	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func getProject(projectService *compute.ProjectsService, projectID string) (*compute.Project, error) {
	project, err := projectService.Get(projectID).Do()
	if err != nil {
		return nil, err
	}
	return project, err
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

func validateUpdateParms(projectID, key, value string) errors.Errors {
	var errors errors.Errors
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
