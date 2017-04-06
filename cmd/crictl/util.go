package main

import (
	"sort"
)

type listOptions struct {
	id string
	// state of the sandbox
	state string
	// quiet is for listing just sandbox IDs
	quiet bool
	// labels are selectors for the sandbox
	labels map[string]string
}

func getSortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
