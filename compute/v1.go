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

// ListInstanceGroups prints all instance groups for the given projectID.
func ListInstanceGroups(projectID string) error {
	if projectID == "" {
		return fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank")
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		return err
	}

	igms := compute.NewInstanceGroupManagersService(computeService)

	list, err := aggregatedList(igms, projectID)
	if err != nil {
		return err
	}

	for k, item := range list.Items {
		if len(item.InstanceGroupManagers) > 0 {
			log.Println(k) // print zone
		}

		for _, v := range item.InstanceGroupManagers {
			log.Println("    ", v.Name) // instance group
			a := strings.Split(v.Zone, "/")

			instances, err := ListManagedInstances(igms, projectID, a[len(a)-1], v.Name)
			if err != nil {
				return err
			}

			for _, instance := range instances {
				log.Println("        ", instance) // instances
			}
		}
	}

	return nil
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
func CompareProjects(projectA, projectB string) (map[string]CompareMeta, error) {
	if projectA == "" {
		return nil, fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank")
	}

	if projectB == "" {
		return nil, fmt.Errorf("-compare project cannot be blank")
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		return nil, err
	}

	projectService := compute.NewProjectsService(computeService)

	aInfo, err := getProject(projectService, projectA)
	if err != nil {
		return nil, err
	}

	bInfo, err := getProject(projectService, projectB)
	if err != nil {
		return nil, err
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

	return keys, nil
}

func PrintKeys(keys map[string]CompareMeta, projectA, projectB string) {
	log.Printf("%-45.45s | %-5.5s | %-25.25s | %-25.25s\n", "key", "equal", projectA, projectB)
	log.Printf("%s\n", strings.Repeat("=", 45+5+2*25+3*3))
	for k := range keys {
		log.Printf("%-45.45s | %5t | %-25.25s | %-25.25s\n", k, keys[k].A == keys[k].B, keys[k].A, keys[k].B)
	}
}

// ListKeys prints a list of all keys associated with a projectID.
func ListKeys(projectID string) error {
	if projectID == "" {
		return fmt.Errorf("GOOGLE_PROJECT_ID cannot be blank")
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		return err
	}

	projectService := compute.NewProjectsService(computeService)

	project, err := getProject(projectService, projectID)
	if err != nil {
		return err
	}

	sort.Sort(ItemsByKey(project.CommonInstanceMetadata.Items))
	log.Printf("%-45.45s | %-30.30s\n", "key", projectID)
	log.Printf("%s\n", strings.Repeat("=", 45+30+1*3))
	for _, meta := range project.CommonInstanceMetadata.Items {
		log.Printf("%-45.45s | %-30.30s\n", meta.Key, *meta.Value)
	}

	return nil
}

// ItemsByKey is a sortable interface for metadata items.
type ItemsByKey []*compute.MetadataItems

func (m ItemsByKey) Len() int           { return len(m) }
func (m ItemsByKey) Less(i, j int) bool { return m[i].Key < m[j].Key }
func (m ItemsByKey) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// UpdateKey updates the projects common metadata key with newValue.
func UpdateKey(projectID string, key string, newValue string) error {
	configErrors := validateUpdateParms(projectID, key, newValue)
	if configErrors != nil {
		return configErrors
	}

	computeService, err := v1ComputeClient()
	if err != nil {
		return err
	}

	projectService := compute.NewProjectsService(computeService)
	// TODO (NF 2018-08-10): Retry loop when a fingerprint doesn't match (aka a concurrent write).
	// retrieve current values
	project, err := getProject(projectService, projectID)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%v-%d.json", project.Name, time.Now().Unix())
	w, err := os.Create(filename)
	if err != nil {
		return err
	}

	err = writeMeta(w, project.CommonInstanceMetadata)
	if err != nil {
		return err
	}

	log.Println("wrote current metadata to", filename)
	err = updateCommonKey(project, key, newValue, projectService)
	if err != nil {
		return err
	}

	return nil
}

func v1ComputeClient() (*compute.Service, error) {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, err
	}

	return compute.New(client)
}

func getProject(projectService *compute.ProjectsService, projectID string) (*compute.Project, error) {
	project, err := projectService.Get(projectID).Do()
	return project, err
}

func writeMeta(w io.WriteCloser, meta *compute.Metadata) error {
	enc := json.NewEncoder(w)
	return enc.Encode(meta)
}

func updateCommonKey(project *compute.Project, key string, newValue string, projectService *compute.ProjectsService) error {
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
		return err
	}
	if op.Error != nil {
		return fmt.Errorf("%+v", op.Error.Errors)
	}

	return nil
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
