package flag_test

import (
	"fmt"

	"zombiezen.com/go/gut/internal/flag"
)

func Example() {
	fset := new(flag.FlagSet)
	x := fset.Bool("x", false, "boolean flag")
	output := fset.String("o", "", "output path (string flag)")
	if err := fset.Parse([]string{"-o", "/path/to/foo", "input", "-x"}); err != nil {
		panic(err)
	}
	fmt.Println("x =", *x)
	fmt.Println("output =", *output)
	fmt.Println("args =", fset.Args())

	// output:
	// x = true
	// output = /path/to/foo
	// args = [input]
}
