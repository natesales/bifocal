package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomNode(t *testing.T) {
	nodes, err := ringNodes()
	assert.Nil(t, err)

	var usedNodes []int
	for i := 0; i < 100; i++ {
		randNode, err := randomNode(nodes, 100)
		assert.Nil(t, err)
		assert.NotContains(t, usedNodes, randNode.Id)
		usedNodes = append(usedNodes, randNode.Id)
	}
}
