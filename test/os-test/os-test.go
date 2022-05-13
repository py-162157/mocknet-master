package main

import (
	"fmt"
	"os/exec"
)

var POD_VPP_CONFIG_FILE = "/home/ubuntu/vpp.conf"
var container_id = "mocknet-h1s1-cb9c7dcfb-4722x"

func main() {
	cmd := exec.Command("kubectl", "exec", container_id, "-n", "default", "--", "ifconfig", "tap0", "|", "awk", `'/ether/'`, "|", "awk", `'{print $2}'`)

	r, err := cmd.Output()
	if err == nil {
		fmt.Println(string(r))
	} else {
		fmt.Println("failed:", err)
	}
}
