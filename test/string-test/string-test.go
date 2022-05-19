package main

import (
	"fmt"
	"strconv"
)

func main() {
	addr := [4]uint8{244, 0, 0, 0}

	fmt.Println(strconv.Itoa(int(addr[0])))
}
