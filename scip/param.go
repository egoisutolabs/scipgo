package scip

import (
	"fmt"
	"math"
	"runtime"
	"unicode"
)

// SetParam sets the value of a SCIP parameter. value may be a bool, int,
// int32, int64, float32, float64, string or byte. A byte sets a char
// parameter. An int32 does too when that parameter is a char and the value
// is ASCII, which is how a rune reaches the char setter: rune is an alias
// of int32, so a character literal and an integer share a type and the
// parameter's SCIP type distinguishes them. A rune outside ASCII for a char
// parameter is rejected with RetcodeInvalidData before SCIP is called. Any
// other int32 is an integer parameter.
func SetParam(m Model, name string, value any) (Model, error) {
	switch v := value.(type) {
	case float64:
		return m.SetRealParam(name, v)
	case float32:
		return m.SetRealParam(name, float64(v))
	case int:
		if v < math.MinInt32 || v > math.MaxInt32 {
			return m, m.invalid("SetIntParam", RetcodeParameterWrongVal, fmt.Sprintf("%s: int value %d overflows int32; pass an int64", name, v))
		}
		return m.SetIntParam(name, int32(v))
	case int32:
		return m.setInt32Param(name, v)
	case int64:
		return m.SetLongintParam(name, v)
	case bool:
		return m.SetBoolParam(name, v)
	case byte:
		return m.SetCharParam(name, v)
	case string:
		return m.SetStrParam(name, v)
	default:
		return m, m.invalid("SetParam", RetcodeInvalidData, fmt.Sprintf("%s: unsupported parameter type %T", name, value))
	}
}

// setInt32Param routes an int32 to the char setter when name is a char
// parameter. Go's rune type is an int32, so this is also the path a
// character literal takes.
func (m Model) setInt32Param(name string, v int32) (Model, error) {
	defer runtime.KeepAlive(m.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if m.scip.alive() && m.scip.isCharParam(name) {
		if v < 0 || v > unicode.MaxASCII {
			return m, m.invalid("SetCharParam", RetcodeInvalidData, fmt.Sprintf("%s: rune %d is not a single ASCII byte", name, v))
		}
		return m.SetCharParam(name, byte(v))
	}
	return m.SetIntParam(name, v)
}

// GetParam returns the value of a SCIP parameter into the pointer given by
// out, which must be a *int32, *int64, *float64, *bool, *string or *byte
// (mirrors russcip's generic param::<T>). A char parameter is read into a
// *byte.
func GetParam(m Model, name string, out any) error {
	switch o := out.(type) {
	case *float64:
		v, err := m.TryRealParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *int32:
		v, err := m.TryIntParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *int64:
		v, err := m.TryLongintParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *bool:
		v, err := m.TryBoolParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *byte:
		v, err := m.TryCharParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *string:
		v, err := m.TryStrParam(name)
		if err != nil {
			return err
		}
		*o = v
	default:
		return m.invalid("GetParam", RetcodeInvalidData, fmt.Sprintf("%s: unsupported parameter out type %T", name, out))
	}
	return nil
}
