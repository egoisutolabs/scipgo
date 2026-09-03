package scip

/*
#include "helpers.h"
*/
import "C"

// Presolver is a wrapper for a SCIP presolver plugin.
type Presolver struct {
	raw *C.SCIP_PRESOL
}

// Inner returns the raw pointer to the underlying SCIP_PRESOL.
func (p Presolver) Inner() *C.SCIP_PRESOL { return p.raw }

// Name returns the name of the presolver.
func (p Presolver) Name() string { return goString(C.SCIPpresolGetName(p.raw)) }

// Desc returns the description of the presolver.
func (p Presolver) Desc() string { return goString(C.SCIPpresolGetDesc(p.raw)) }

// Priority returns the priority of the presolver.
func (p Presolver) Priority() int32 { return int32(C.SCIPpresolGetPriority(p.raw)) }

// MaxRounds returns the maximal number of presolving rounds the presolver
// participates in; -1 means no limit.
func (p Presolver) MaxRounds() int32 { return int32(C.SCIPpresolGetMaxrounds(p.raw)) }

// NCalls returns the number of times the presolver was called.
func (p Presolver) NCalls() int { return int(C.SCIPpresolGetNCalls(p.raw)) }

// Time returns the time spent in the presolver, in seconds.
func (p Presolver) Time() float64 { return float64(C.SCIPpresolGetTime(p.raw)) }
