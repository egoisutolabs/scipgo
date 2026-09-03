// An event handler that prints info about every focused node and checks
// that the number of NODE_FOCUSED events matches the total node count.
package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/egoisutolabs/scipgo/scip"
)

type nodeInfoEventHandler struct {
	mu   sync.Mutex
	runs int
}

func (h *nodeInfoEventHandler) GetEventMask() scip.EventMask {
	// Listen for node events (when nodes are focused)
	return scip.EventMaskNodeFocused
}

func (h *nodeInfoEventHandler) Execute(model scip.Model, _ scip.SCIPEventhdlr, _ scip.Event) {
	currentNode := model.FocusNode()
	h.mu.Lock()
	h.runs++
	h.mu.Unlock()

	parent := "none"
	if p, ok := currentNode.Parent(); ok {
		parent = fmt.Sprintf("%d", p.Number())
	}
	fmt.Printf("-- NodeInfoEventHandler: at Node %d: depth = %d, parent = %s\n",
		currentNode.Number(), currentNode.Depth(), parent)
}

func main() {
	model, err := scip.NewModel().
		HideOutput().
		IncludeDefaultPlugins().
		ReadProb("../../data/test/simple.mps")
	if err != nil {
		fmt.Println("failed to read problem:", err)
		os.Exit(1)
	}
	model, err = scip.SetParam(model.
		SetHeuristics(scip.ParamSettingOff).
		SetSeparating(scip.ParamSettingOff).
		SetPresolving(scip.ParamSettingOff), "branching/pscost/priority", int32(1000000))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	handler := &nodeInfoEventHandler{}
	model.Add(scip.NewEventhdlr(handler).Name("NodeInfoPrinter"))

	solved := model.Solve()
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.runs == 0 {
		fmt.Println("no nodes were seen")
		os.Exit(1)
	}
	if solved.NNodes() != handler.runs {
		fmt.Printf("node count mismatch: %d nodes solved, %d seen\n", solved.NNodes(), handler.runs)
		os.Exit(1)
	}
	fmt.Printf("-- NodeInfoEventHandler: Total nodes seen = %d\n", handler.runs)
}
