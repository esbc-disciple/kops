/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/klog/v2"

	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
)

// DescribeRouteTables lists route-tables tagged for the cluster (shared and owned)
func DescribeRouteTables(cloud fi.Cloud, clusterName string) (map[string]*ec2.RouteTable, error) {
	c := cloud.(awsup.AWSCloud)

	routeTables := make(map[string]*ec2.RouteTable)
	klog.V(2).Info("Listing EC2 RouteTables")
	for _, filters := range buildEC2FiltersForCluster(clusterName) {
		request := &ec2.DescribeRouteTablesInput{
			Filters: filters,
		}
		response, err := c.EC2().DescribeRouteTables(request)
		if err != nil {
			return nil, fmt.Errorf("error listing RouteTables: %v", err)
		}

		for _, rt := range response.RouteTables {
			routeTables[aws.ToString(rt.RouteTableId)] = rt
		}
	}

	return routeTables, nil
}

func ListRouteTables(cloud fi.Cloud, vpcID, clusterName string) ([]*resources.Resource, error) {
	routeTables, err := DescribeRouteTables(cloud, clusterName)
	if err != nil {
		return nil, err
	}

	var resourceTrackers []*resources.Resource

	for _, rt := range routeTables {
		resourceTracker := buildTrackerForRouteTable(rt, clusterName)
		resourceTrackers = append(resourceTrackers, resourceTracker)
	}

	return resourceTrackers, nil
}

func dumpRouteTable(op *resources.DumpOperation, r *resources.Resource) error {
	data := make(map[string]interface{})
	data["id"] = r.ID
	data["type"] = r.Type
	data["raw"] = r.Obj
	op.Dump.Resources = append(op.Dump.Resources, data)
	return nil
}

func buildTrackerForRouteTable(rt *ec2.RouteTable, clusterName string) *resources.Resource {
	resourceTracker := &resources.Resource{
		Name:    FindName(rt.Tags),
		ID:      aws.ToString(rt.RouteTableId),
		Type:    ec2.ResourceTypeRouteTable,
		Obj:     rt,
		Dumper:  dumpRouteTable,
		Deleter: DeleteRouteTable,
		Shared:  !HasOwnedTag(ec2.ResourceTypeRouteTable+":"+*rt.RouteTableId, rt.Tags, clusterName),
	}

	var blocks []string
	var blocked []string

	blocks = append(blocks, "vpc:"+aws.ToString(rt.VpcId))

	for _, a := range rt.Associations {
		blocked = append(blocked, "subnet:"+aws.ToString(a.SubnetId))
	}

	resourceTracker.Blocks = blocks
	resourceTracker.Blocked = blocked

	return resourceTracker
}
