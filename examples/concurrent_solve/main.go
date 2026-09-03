// Solve a MIPLIB instance with SCIP's concurrent solvers, using one thread
// per CPU core. Pass "deterministic" as the first argument for deterministic
// mode; anything else uses the default opportunistic mode.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/egoisutolabs/scipgo/scip"
)

func main() {
	deterministic := len(os.Args) > 1 && os.Args[1] == "deterministic"

	model, err := scip.NewModel().
		IncludeDefaultPlugins().
		ReadProb("../../data/test/p0201.mps")
	if err != nil {
		fmt.Println("failed to read problem:", err)
		os.Exit(1)
	}

	nThreads := int32(runtime.NumCPU())
	mode := int32(0)
	if deterministic {
		mode = 1
	}
	modeName := "opportunistic"
	if deterministic {
		modeName = "deterministic"
	}
	fmt.Printf("Solving with %d threads in %s mode\n", nThreads, modeName)

	model, err = model.SetIntParam("parallel/maxnthreads", nThreads)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	model, err = model.SetIntParam("parallel/mode", mode)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	solved, err := model.TrySolveConcurrent()
	if err != nil {
		fmt.Println("concurrent solve failed (was SCIP built with thread support?):", err)
		os.Exit(1)
	}

	fmt.Println("Solved with status", solved.Status())
	fmt.Println("Objective value:", solved.ObjVal())
}
