package scip

import (
	"fmt"
	"math"
)

// SetParam sets the value of a SCIP parameter; the value can be an int32,
// int64, float64, bool or string (mirrors russcip's generic set_param).
func SetParam(m Model, name string, value any) (Model, error) {
	switch v := value.(type) {
	case float64:
		return m.SetRealParam(name, v)
	case float32:
		return m.SetRealParam(name, float64(v))
	case int:
		if v < math.MinInt32 || v > math.MaxInt32 {
			return m, fmt.Errorf("scip: int value %d for parameter %q overflows int32; pass an int64", v, name)
		}
		return m.SetIntParam(name, int32(v))
	case int32:
		return m.SetIntParam(name, v)
	case int64:
		return m.SetLongintParam(name, v)
	case bool:
		return m.SetBoolParam(name, v)
	case string:
		return m.SetStrParam(name, v)
	default:
		return m, fmt.Errorf("scip: unsupported parameter type %T", value)
	}
}

// GetParam returns the value of a SCIP parameter into the pointer given by
// out, which must be a *int32, *int64, *float64, *bool or *string (mirrors
// russcip's generic param::<T>).
func GetParam(m Model, name string, out any) error {
	switch o := out.(type) {
	case *float64:
		v, err := m.scip.realParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *int32:
		v, err := m.scip.intParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *int64:
		v, err := m.scip.longintParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *bool:
		v, err := m.scip.boolParam(name)
		if err != nil {
			return err
		}
		*o = v
	case *string:
		v, err := m.scip.strParam(name)
		if err != nil {
			return err
		}
		*o = v
	default:
		return fmt.Errorf("scip: unsupported parameter out type %T", out)
	}
	return nil
}
