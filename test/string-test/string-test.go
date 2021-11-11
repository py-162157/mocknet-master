package main

import (
	"fmt"
)

func main() {
	value := ""
	value = value + "node1: " + "link.Node1.Name" + ", "
	value = value + "node2: " + "link.Node2.Name" + ", "
	value = value + "node1_inf: " + "link.Node1Inf" + ", "
	value = value + "node2_inf: " + "link.Node2Inf"

	fmt.Println(value)
}
