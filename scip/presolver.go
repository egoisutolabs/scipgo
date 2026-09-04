package scip

/*
#include "helpers.h"
*/
import "C"

// PresolverPlugin is a wrapper for a SCIP presolver plugin.
type PresolverPlugin struct {
	raw  *C.SCIP_PRESOL
	scip *Scip // keeps the owning instance alive and identifies it
}

// live panics with *Error unless the wrapper is usable; see handleErr.
func (h PresolverPlugin) live(op string) {
	mustLive(op, "PresolverPlugin", h.raw != nil, h.scip, 0, true)
}

// Inner returns the raw pointer to the underlying SCIP_PRESOL.
func (p PresolverPlugin) Inner() *C.SCIP_PRESOL { return p.raw }

// Name returns the name of the presolver.
func (p PresolverPlugin) Name() string {
	p.live("PresolverPlugin.Name")
	return goString(C.SCIPpresolGetName(p.raw))
}

// Desc returns the description of the presolver.
func (p PresolverPlugin) Desc() string {
	p.live("PresolverPlugin.Desc")
	return goString(C.SCIPpresolGetDesc(p.raw))
}

// Priority returns the priority of the presolver.
func (p PresolverPlugin) Priority() int32 {
	p.live("PresolverPlugin.Priority")
	return int32(C.SCIPpresolGetPriority(p.raw))
}

// MaxRounds returns the maximal number of presolving rounds the presolver
// participates in; -1 means no limit.
func (p PresolverPlugin) MaxRounds() int32 {
	p.live("PresolverPlugin.MaxRounds")
	return int32(C.SCIPpresolGetMaxrounds(p.raw))
}

// NCalls returns the number of times the presolver was called.
func (p PresolverPlugin) NCalls() int {
	p.live("PresolverPlugin.NCalls")
	return int(C.SCIPpresolGetNCalls(p.raw))
}

// Time returns the time spent in the presolver, in seconds.
func (p PresolverPlugin) Time() float64 {
	p.live("PresolverPlugin.Time")
	return float64(C.SCIPpresolGetTime(p.raw))
}
