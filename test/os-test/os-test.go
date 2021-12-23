package main

import (
	"fmt"
	"os/exec"
)

var POD_VPP_CONFIG_FILE = "/home/ubuntu/vpp.conf"

func main() {
	create_cmd := exec.Command("kubectl", "exec", "-it", "mocknet-h1-6c64d8699f-qkr2t", "--", "vppctl", "-s", ":5002", "create", "tap")
	output, err := create_cmd.Output()
	if err != nil {
		fmt.Println("err:", err)
	} else {
		fmt.Println("success:", string(output))
	}
}
