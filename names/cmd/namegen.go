package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/openSUSE/piirplug/names"
)

func main() {
	lengthFlag := flag.Int("length", 10, "length of the generated name (1-255)")
	countFlag := flag.Int("count", 1, "number of names to generate")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Generates random strings.\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *lengthFlag <= 0 || *lengthFlag > 255 {
		fmt.Fprintf(os.Stderr, "Error: length must be between 1 and 255\n")
		os.Exit(1)
	}

	if *countFlag <= 0 {
		fmt.Fprintf(os.Stderr, "Error: count must be greater than 0\n")
		os.Exit(1)
	}

	for i := 0; i < *countFlag; i++ {
		name, err := names.GenerateName(names.WithLength(*lengthFlag))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating name: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(name)
	}
}
