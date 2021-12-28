package main

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
)

var usedNodes []int

type Node struct {
	Id          int    `json:"id"`
	Active      int    `json:"active"`
	Geo         string `json:"geo"`
	Datacenter  string `json:"datacenter"`
	Participant int    `json:"participant"`
	CountryCode string `json:"countrycode"`
	Hostname    string `json:"hostname"`
	ASN         int    `json:"asn"`
	IPv4        string `json:"ipv4"`
	IPv6        string `json:"ipv6"`
}

type NodesResponse struct {
	Info struct {
		Success int `json:"success"`
	} `json:"info"`
	Results struct {
		Nodes []Node `json:"nodes"`
	} `json:"results"`
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ringNodes() ([]Node, error) {
	res, err := http.Get("https://api.ring.nlnog.net/1.0/nodes/active")
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var nodesResponse NodesResponse
	if err := json.Unmarshal(body, &nodesResponse); err != nil {
		return nil, err
	}
	return nodesResponse.Results.Nodes, nil
}

// randomNode gets a random ring node that hasn't been used in the last n queries
func randomNode(nodes []Node, n int) (*Node, error) {
	if len(usedNodes) == n {
		usedNodes = usedNodes[1:]
	}

	for {
		randomNode := nodes[rand.Intn(len(nodes))]
		if !contains(usedNodes, randomNode.Id) {
			usedNodes = append(usedNodes, randomNode.Id)
			return &randomNode, nil
		}
	}
}
