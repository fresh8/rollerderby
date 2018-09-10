package compute

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v0.beta"
)

// RollingReplace replaces all of the instances rolling style.
func RollingReplace(projectID, zone, groupName string, minReadySec int64) {
	computeService, err := betaComputeClient()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	igms := computeService.InstanceGroupManagers

	policy, err := igms.Get(projectID, zone, groupName).Do()
	if err != nil {
		log.Fatal(err)
	}

	nextVer := &compute.InstanceGroupManagerVersion{
		Name:             fmt.Sprintf("0-%v", time.Now().Unix()),
		InstanceTemplate: policy.InstanceTemplate,
	}

	policy.Versions[0] = nextVer
	policy.UpdatePolicy.MinimalAction = "REPLACE"
	policy.UpdatePolicy.MinReadySec = minReadySec

	op, err := igms.Patch(projectID, zone, groupName, policy).Do()
	if err != nil {
		log.Fatal(err)
	}

	if op.Error != nil {
		for _, e := range op.Error.Errors {
			log.Println(e.Code, e.Message, e.Location)
		}
		log.Fatalf("got op.Error != nil, want nil")
	}

	log.Printf("replacing group %v", groupName)
}

func betaComputeClient() (*compute.Service, error) {
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
