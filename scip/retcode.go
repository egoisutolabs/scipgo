package scip

/*
#include "helpers.h"
*/
import "C"

// Retcode represents the possible return codes from SCIP functions.
type Retcode int

// SCIP return codes.
const (
	RetcodeOkay               Retcode = iota // Normal termination
	RetcodeError                             // Unspecified error
	RetcodeNoMemory                          // Insufficient memory error
	RetcodeReadError                         // Read error
	RetcodeWriteError                        // Write error
	RetcodeNoFile                            // File not found error
	RetcodeFileCreateError                   // Cannot create file
	RetcodeLpError                           // Error in LP solver
	RetcodeNoProblem                         // No problem exists
	RetcodeInvalidCall                       // Method cannot be called at this time
	RetcodeInvalidData                       // Error in input data
	RetcodeInvalidResult                     // Method returned an invalid result code
	RetcodePluginNotFound                    // A required plugin was not found
	RetcodeParameterUnknown                  // Parameter with the given name was not found
	RetcodeParameterWrongType                // Parameter is not of the expected type
	RetcodeParameterWrongVal                 // Value is invalid for the given parameter
	RetcodeKeyAlreadyExisting                // Given key is already existing in table
	RetcodeMaxDepthLevel                     // Maximal branching depth level exceeded
	RetcodeBranchError                       // No branching could be created
	RetcodeNotImplemented                    // Function not implemented
	RetcodeUnknown                           // Any status code not specifically represented
)

// Error implements the error interface for Retcode.
func (r Retcode) Error() string {
	names := map[Retcode]string{
		RetcodeOkay:               "SCIP_OKAY",
		RetcodeError:              "SCIP_ERROR",
		RetcodeNoMemory:           "SCIP_NOMEMORY",
		RetcodeReadError:          "SCIP_READERROR",
		RetcodeWriteError:         "SCIP_WRITEERROR",
		RetcodeNoFile:             "SCIP_NOFILE",
		RetcodeFileCreateError:    "SCIP_FILECREATEERROR",
		RetcodeLpError:            "SCIP_LPERROR",
		RetcodeNoProblem:          "SCIP_NOPROBLEM",
		RetcodeInvalidCall:        "SCIP_INVALIDCALL",
		RetcodeInvalidData:        "SCIP_INVALIDDATA",
		RetcodeInvalidResult:      "SCIP_INVALIDRESULT",
		RetcodePluginNotFound:     "SCIP_PLUGINNOTFOUND",
		RetcodeParameterUnknown:   "SCIP_PARAMETERUNKNOWN",
		RetcodeParameterWrongType: "SCIP_PARAMETERWRONGTYPE",
		RetcodeParameterWrongVal:  "SCIP_PARAMETERWRONGVAL",
		RetcodeKeyAlreadyExisting: "SCIP_KEYALREADYEXISTING",
		RetcodeMaxDepthLevel:      "SCIP_MAXDEPTHLEVEL",
		RetcodeBranchError:        "SCIP_BRANCHERROR",
		RetcodeNotImplemented:     "SCIP_NOTIMPLEMENTED",
		RetcodeUnknown:            "SCIP_UNKNOWN_RETCODE",
	}
	if name, ok := names[r]; ok {
		return name
	}
	return "SCIP_UNKNOWN_RETCODE"
}

// String implements fmt.Stringer.
func (r Retcode) String() string { return r.Error() }

func retcodeFromC(val C.SCIP_RETCODE) Retcode {
	switch val {
	case C.SCIP_OKAY:
		return RetcodeOkay
	case C.SCIP_ERROR:
		return RetcodeError
	case C.SCIP_NOMEMORY:
		return RetcodeNoMemory
	case C.SCIP_READERROR:
		return RetcodeReadError
	case C.SCIP_WRITEERROR:
		return RetcodeWriteError
	case C.SCIP_NOFILE:
		return RetcodeNoFile
	case C.SCIP_FILECREATEERROR:
		return RetcodeFileCreateError
	case C.SCIP_LPERROR:
		return RetcodeLpError
	case C.SCIP_NOPROBLEM:
		return RetcodeNoProblem
	case C.SCIP_INVALIDCALL:
		return RetcodeInvalidCall
	case C.SCIP_INVALIDDATA:
		return RetcodeInvalidData
	case C.SCIP_INVALIDRESULT:
		return RetcodeInvalidResult
	case C.SCIP_PLUGINNOTFOUND:
		return RetcodePluginNotFound
	case C.SCIP_PARAMETERUNKNOWN:
		return RetcodeParameterUnknown
	case C.SCIP_PARAMETERWRONGTYPE:
		return RetcodeParameterWrongType
	case C.SCIP_PARAMETERWRONGVAL:
		return RetcodeParameterWrongVal
	case C.SCIP_KEYALREADYEXISTING:
		return RetcodeKeyAlreadyExisting
	case C.SCIP_MAXDEPTHLEVEL:
		return RetcodeMaxDepthLevel
	case C.SCIP_BRANCHERROR:
		return RetcodeBranchError
	case C.SCIP_NOTIMPLEMENTED:
		return RetcodeNotImplemented
	default:
		return RetcodeUnknown
	}
}

func retcodeToC(r Retcode) C.SCIP_RETCODE {
	switch r {
	case RetcodeOkay:
		return C.SCIP_OKAY
	case RetcodeError:
		return C.SCIP_ERROR
	case RetcodeNoMemory:
		return C.SCIP_NOMEMORY
	case RetcodeReadError:
		return C.SCIP_READERROR
	case RetcodeWriteError:
		return C.SCIP_WRITEERROR
	case RetcodeNoFile:
		return C.SCIP_NOFILE
	case RetcodeFileCreateError:
		return C.SCIP_FILECREATEERROR
	case RetcodeLpError:
		return C.SCIP_LPERROR
	case RetcodeNoProblem:
		return C.SCIP_NOPROBLEM
	case RetcodeInvalidCall:
		return C.SCIP_INVALIDCALL
	case RetcodeInvalidData:
		return C.SCIP_INVALIDDATA
	case RetcodeInvalidResult:
		return C.SCIP_INVALIDRESULT
	case RetcodePluginNotFound:
		return C.SCIP_PLUGINNOTFOUND
	case RetcodeParameterUnknown:
		return C.SCIP_PARAMETERUNKNOWN
	case RetcodeParameterWrongType:
		return C.SCIP_PARAMETERWRONGTYPE
	case RetcodeParameterWrongVal:
		return C.SCIP_PARAMETERWRONGVAL
	case RetcodeKeyAlreadyExisting:
		return C.SCIP_KEYALREADYEXISTING
	case RetcodeMaxDepthLevel:
		return C.SCIP_MAXDEPTHLEVEL
	case RetcodeBranchError:
		return C.SCIP_BRANCHERROR
	case RetcodeNotImplemented:
		return C.SCIP_NOTIMPLEMENTED
	default:
		return C.SCIP_ERROR
	}
}
