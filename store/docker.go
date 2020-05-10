package store

// swarm-dns-sd
// Copyright (C) 2020 Maximilian Pachl

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// ---------------------------------------------------------------------------------------
//  imports
// ---------------------------------------------------------------------------------------

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

// ---------------------------------------------------------------------------------------
//  constants
// ---------------------------------------------------------------------------------------

const (
	LabelGroup = "kallax.group"
)

// ---------------------------------------------------------------------------------------
//  types
// ---------------------------------------------------------------------------------------

type docker struct {
	client *client.Client

	nodesNames     map[string]string
	nodeNamesMutex sync.Mutex
}

// ---------------------------------------------------------------------------------------
//  public functions
// ---------------------------------------------------------------------------------------

// NewDocker constructs a new Store backed by a docker Swarm cluster.
func NewDocker(ops ...client.Opt) (Store, error) {
	d := docker{
		nodesNames: make(map[string]string),
	}

	var err error
	d.client, err = client.NewClientWithOpts(ops...)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// ---------------------------------------------------------------------------------------
//  public members
// ---------------------------------------------------------------------------------------

// GetGroupEndpoints returns all Endpoints which belong to the given group.
func (d *docker) GetGroupEndpoints(group string) ([]*Endpoint, error) {
	groupLabel := LabelGroup + "." + group

	// find all swarm services with the group label
	filter := filters.NewArgs()
	filter.Add("label", groupLabel)
	services, err := d.client.ServiceList(context.Background(), types.ServiceListOptions{
		Filters: filter,
	})
	if err != nil {
		return nil, err
	}

	endpoints := make([]*Endpoint, 0)

	for _, service := range services {
		// parse endpoint specification from swarm label
		var endpointSpecs map[string]*EndpointSpec
		err = json.Unmarshal([]byte(service.Spec.Labels[groupLabel]), &endpointSpecs)
		if err != nil {
			return nil, err
		}

		// find all swarm tasks beloging to the service
		filter := filters.NewArgs()
		filter.Add("service", service.ID)
		tasks, err := d.client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		if err != nil {
			return nil, err
		}

		for _, task := range tasks {
			// we are only interested in running tasks
			// other tasks cannot be connected to
			if task.Status.State != swarm.TaskStateRunning {
				continue
			}

			// convert the node ID to a user readable name
			nodeName, err := d.getNodeName(task.NodeID)
			if err != nil {
				logrus.Errorln("failed to query node name:", err.Error())
				nodeName = task.NodeID
			}

			for epName, epSpec := range endpointSpecs {
				endpoints = append(endpoints, &Endpoint{
					Name: fmt.Sprintf("%s.task-%d-%s.%s.%s.%s",
						epName, task.Slot, task.ID, service.Spec.Name,
						nodeName, epSpec.Network),
					Port: epSpec.Port,
				})
			}
		}
	}

	return endpoints, nil
}

func (d *docker) GetTaskIpAddresses(taskId string, networkId string) (string, error) {
	task, _, err := d.client.TaskInspectWithRaw(context.Background(), taskId)
	if err != nil {
		return "", err
	}

	ip := ""
	for _, network := range task.NetworksAttachments {
		if network.Network.ID == networkId && len(network.Addresses) > 0 {
			addr, _, err := net.ParseCIDR(network.Addresses[0])
			if err != nil {
				return "", err
			}

			ip = addr.String()
		}
	}

	if ip == "" {
		return "", fmt.Errorf("task \"%s\" is not attached to network \"%s\"",
			taskId, networkId)
	}

	return ip, nil
}

// ---------------------------------------------------------------------------------------
//  private members
// ---------------------------------------------------------------------------------------

// getNodeName returns the name of a swarm nodeby its ID.
func (d *docker) getNodeName(nodeId string) (string, error) {
	d.nodeNamesMutex.Lock()
	defer d.nodeNamesMutex.Unlock()

	nodeName, ok := d.nodesNames[nodeId]
	if !ok {
		node, _, err := d.client.NodeInspectWithRaw(context.Background(), nodeId)
		if err != nil {
			return "", err
		}

		nodeName = node.Description.Hostname
		d.nodesNames[nodeId] = node.Description.Hostname
	}

	return nodeName, nil
}
