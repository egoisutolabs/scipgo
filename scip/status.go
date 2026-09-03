package scip

/*
#include "helpers.h"
*/
import "C"

// Status represents the status of a SCIP optimization run.
type Status int

// Solving statuses.
const (
	StatusUnknown           Status = iota // The solving status is not yet known
	StatusUserInterrupt                   // The user interrupted the solving process
	StatusNodeLimit                       // The node limit was reached
	StatusTotalNodeLimit                  // The total node limit was reached (incl. restarts)
	StatusStallNodeLimit                  // The stalling node limit was reached
	StatusTimeLimit                       // The time limit was reached
	StatusMemoryLimit                     // The memory limit was reached
	StatusGapLimit                        // The gap limit was reached
	StatusPrimalLimit                     // The primal limit was reached
	StatusDualLimit                       // The dual limit was reached
	StatusSolutionLimit                   // The solution limit was reached
	StatusBestSolutionLimit               // The solution improvement limit was reached
	StatusRestartLimit                    // The restart limit was reached
	StatusOptimal                         // Solved to optimality
	StatusInfeasible                      // Proven infeasible
	StatusUnbounded                       // Proven unbounded
	StatusInforunbd                       // Proven infeasible or unbounded
	StatusTerminate                       // Process received a SIGTERM signal
)

// String implements fmt.Stringer.
func (s Status) String() string {
	names := [...]string{
		"Unknown", "UserInterrupt", "NodeLimit", "TotalNodeLimit", "StallNodeLimit",
		"TimeLimit", "MemoryLimit", "GapLimit", "PrimalLimit", "DualLimit",
		"SolutionLimit", "BestSolutionLimit", "RestartLimit", "Optimal", "Infeasible",
		"Unbounded", "Inforunbd", "Terminate",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return "Unknown"
}

func statusFromC(val C.SCIP_STATUS) Status {
	switch val {
	case C.SCIP_STATUS_UNKNOWN:
		return StatusUnknown
	case C.SCIP_STATUS_USERINTERRUPT:
		return StatusUserInterrupt
	case C.SCIP_STATUS_NODELIMIT:
		return StatusNodeLimit
	case C.SCIP_STATUS_TOTALNODELIMIT:
		return StatusTotalNodeLimit
	case C.SCIP_STATUS_STALLNODELIMIT:
		return StatusStallNodeLimit
	case C.SCIP_STATUS_TIMELIMIT:
		return StatusTimeLimit
	case C.SCIP_STATUS_MEMLIMIT:
		return StatusMemoryLimit
	case C.SCIP_STATUS_GAPLIMIT:
		return StatusGapLimit
	case C.SCIP_STATUS_PRIMALLIMIT:
		return StatusPrimalLimit
	case C.SCIP_STATUS_DUALLIMIT:
		return StatusDualLimit
	case C.SCIP_STATUS_SOLLIMIT:
		return StatusSolutionLimit
	case C.SCIP_STATUS_BESTSOLLIMIT:
		return StatusBestSolutionLimit
	case C.SCIP_STATUS_RESTARTLIMIT:
		return StatusRestartLimit
	case C.SCIP_STATUS_OPTIMAL:
		return StatusOptimal
	case C.SCIP_STATUS_INFEASIBLE:
		return StatusInfeasible
	case C.SCIP_STATUS_UNBOUNDED:
		return StatusUnbounded
	case C.SCIP_STATUS_INFORUNBD:
		return StatusInforunbd
	case C.SCIP_STATUS_TERMINATE:
		return StatusTerminate
	default:
		panic("unknown SCIP status")
	}
}

// LPStatus represents the status of the LP solver.
type LPStatus int

// LP solver statuses.
const (
	LPStatusNotSolved  LPStatus = iota // The LP is not solved yet
	LPStatusOptimal                    // The LP is solved to optimality
	LPStatusInfeasible                 // The LP is infeasible
	LPStatusUnbounded                  // The LP is unbounded
	LPStatusError                      // Error in solving the LP
	LPStatusIterLimit                  // The iteration limit is reached
	LPStatusObjLimit                   // The objective limit is reached
	LPStatusTimeLimit                  // The time limit is reached
)

// String implements fmt.Stringer.
func (s LPStatus) String() string {
	names := [...]string{"NotSolved", "Optimal", "Infeasible", "Unbounded", "Error", "IterLimit", "ObjLimit", "TimeLimit"}
	if int(s) < len(names) {
		return names[s]
	}
	return "Error"
}

func lpStatusFromC(val C.SCIP_LPSOLSTAT) LPStatus {
	switch val {
	case C.SCIP_LPSOLSTAT_OPTIMAL:
		return LPStatusOptimal
	case C.SCIP_LPSOLSTAT_INFEASIBLE:
		return LPStatusInfeasible
	case C.SCIP_LPSOLSTAT_UNBOUNDEDRAY:
		return LPStatusUnbounded
	case C.SCIP_LPSOLSTAT_NOTSOLVED:
		return LPStatusNotSolved
	case C.SCIP_LPSOLSTAT_ERROR:
		return LPStatusError
	case C.SCIP_LPSOLSTAT_ITERLIMIT:
		return LPStatusIterLimit
	case C.SCIP_LPSOLSTAT_OBJLIMIT:
		return LPStatusObjLimit
	case C.SCIP_LPSOLSTAT_TIMELIMIT:
		return LPStatusTimeLimit
	default:
		return LPStatusError
	}
}

// ObjSense represents the objective sense of an optimization model.
type ObjSense int

// Objective senses.
const (
	ObjSenseMinimize ObjSense = iota // The problem is a minimization problem
	ObjSenseMaximize                 // The problem is a maximization problem
)

func (s ObjSense) toC() C.SCIP_OBJSENSE {
	if s == ObjSenseMaximize {
		return C.SCIP_OBJSENSE_MAXIMIZE
	}
	return C.SCIP_OBJSENSE_MINIMIZE
}

// ParamSetting represents the possible settings for a SCIP parameter.
type ParamSetting int

// Parameter settings.
const (
	ParamSettingDefault    ParamSetting = iota // Use default values
	ParamSettingAggressive                     // Set to aggressive settings
	ParamSettingFast                           // Set to fast settings
	ParamSettingOff                            // Turn off
)

func (p ParamSetting) toC() C.SCIP_PARAMSETTING {
	switch p {
	case ParamSettingDefault:
		return C.SCIP_PARAMSETTING_DEFAULT
	case ParamSettingAggressive:
		return C.SCIP_PARAMSETTING_AGGRESSIVE
	case ParamSettingFast:
		return C.SCIP_PARAMSETTING_FAST
	default:
		return C.SCIP_PARAMSETTING_OFF
	}
}
