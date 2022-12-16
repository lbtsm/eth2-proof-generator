package main

import (
	"fmt"
	"github.com/lbtsm/eth2-proof-generator/proof"
)

func main() {
	fmt.Println(proof.Generate(1, "test"))
}
