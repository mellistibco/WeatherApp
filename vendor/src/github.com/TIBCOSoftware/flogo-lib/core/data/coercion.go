package data

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CoerceToValue coerce a value to the specified type
func CoerceToValue(value interface{}, dataType Type) (interface{}, error) {

	var coerced interface{}
	var err error

	switch dataType {
	case STRING:
		coerced, err = CoerceToString(value)
	case INTEGER:
		coerced, err = CoerceToInteger(value)
	case NUMBER:
		coerced, err = CoerceToNumber(value)
	case BOOLEAN:
		coerced, err = CoerceToBoolean(value)
	case OBJECT:
		coerced, err = CoerceToObject(value)
	case ARRAY:
		coerced, err = CoerceToArray(value)
	case PARAMS:
		coerced, err = CoerceToParams(value)
	case COMPLEX_OBJECT:
		coerced, err = CoerceToComplexObject(value)
	case ANY:
		coerced, err = CoerceToAny(value)
	}

	if err != nil {
		return nil, err
	}

	return coerced, nil
}

//todo check int64,float64 on raspberry pi

// CoerceToString coerce a value to a string
func CoerceToString(val interface{}) (string, error) {
	switch t := val.(type) {
	case string:
		return t, nil
	case int:
		return strconv.Itoa(t), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case json.Number:
		return t.String(), nil
	case bool:
		return strconv.FormatBool(t), nil
	case nil:
		return "", nil
	case map[string]interface{}:
		b, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("Unable to Coerce %#v to string", t)
	}
}

// CoerceToInteger coerce a value to an integer
func CoerceToInteger(val interface{}) (int, error) {
	switch t := val.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case json.Number:
		i, err := t.Int64()
		return int(i), err
	case string:
		return strconv.Atoi(t)
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("Unable to coerce %#v to integer", val)
	}
}

// CoerceToNumber coerce a value to a number
func CoerceToNumber(val interface{}) (float64, error) {
	switch t := val.(type) {
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case float64:
		return t, nil
	case json.Number:
		return t.Float64()
	case string:
		return strconv.ParseFloat(t, 64)
	case bool:
		if t {
			return 1.0, nil
		}
		return 0.0, nil
	case nil:
		return 0.0, nil
	default:
		return 0.0, fmt.Errorf("Unable to coerce %#v to float", val)
	}
}

// CoerceToBoolean coerce a value to a boolean
func CoerceToBoolean(val interface{}) (bool, error) {
	switch t := val.(type) {
	case bool:
		return t, nil
	case int:
		return t != 0, nil
	case int64:
		return t != 0, nil
	case float64:
		return t != 0.0, nil
	case json.Number:
		i, err := t.Int64()
		return i != 0, err
	case string:
		return strconv.ParseBool(t)
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("Unable to coerce %#v to bool", val)
	}
}

// CoerceToObject coerce a value to an object
func CoerceToObject(val interface{}) (map[string]interface{}, error) {

	switch t := val.(type) {
	case map[string]interface{}:
		return t, nil
	default:
		return nil, fmt.Errorf("Unable to coerce %#v to map[string]interface{}", val)
	}
}

// CoerceToArray coerce a value to an array
func CoerceToArray(val interface{}) ([]interface{}, error) {

	switch t := val.(type) {
	case []interface{}:
		return t, nil
	case []map[string]interface{}:
		var a []interface{}
		for _, v := range t {
			a = append(a, v)
		}
		return a, nil
	default:
		return nil, fmt.Errorf("Unable to coerce %#v to []interface{}", val)
	}
}

// CoerceToArray coerce a value to an array
func CoerceToAny(val interface{}) (interface{}, error) {

	switch t := val.(type) {

	case json.Number:

		if strings.Contains(t.String(), ".") {
			return t.Float64()
		} else {
			return t.Int64()
		}
	default:
		return val, nil
	}
}

// CoerceToParams coerce a value to params
func CoerceToParams(val interface{}) (map[string]string, error) {

	switch t := val.(type) {
	case map[string]string:
		return t, nil
	case map[string]interface{}:

		var m = make(map[string]string, len(t))
		for k, v := range t {

			mVal, err := CoerceToString(v)
			if err != nil {
				return nil, err
			}
			m[k] = mVal
		}
		return m, nil
	case map[interface{}]string:

		var m = make(map[string]string, len(t))
		for k, v := range t {

			mKey, err := CoerceToString(k)
			if err != nil {
				return nil, err
			}
			m[mKey] = v
		}
		return m, nil
	case map[interface{}]interface{}:

		var m = make(map[string]string, len(t))
		for k, v := range t {

			mKey, err := CoerceToString(k)
			if err != nil {
				return nil, err
			}

			mVal, err := CoerceToString(v)
			if err != nil {
				return nil, err
			}
			m[mKey] = mVal
		}
		return m, nil
	default:
		return nil, fmt.Errorf("Unable to coerce %#v to map[string]string", val)
	}
}

// CoerceToObject coerce a value to an complex object
func CoerceToComplexObject(val interface{}) (*ComplexObject, error) {
	//If the val is nil then just return empty struct
	var emptyComplexObject = &ComplexObject{Value: "{}"}
	if val == nil {
		return emptyComplexObject, nil
	}
	switch t := val.(type) {
	case string:
		if val == "" {
			return emptyComplexObject, nil
		} else {
			complexObject := &ComplexObject{}
			err := json.Unmarshal([]byte(t), complexObject)
			if err != nil {
				return nil, err

			}
			return handleComplex(complexObject), nil
		}
	case map[string]interface{}:
		v, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		complexObject := &ComplexObject{}
		err = json.Unmarshal(v, complexObject)
		if err != nil {
			return nil, err
		}
		return handleComplex(complexObject), nil
	case *ComplexObject:
		return handleComplex(val.(*ComplexObject)), nil
	default:
		return nil, fmt.Errorf("Unable to coerce %#v to complex object", val)
	}
}

func handleComplex(complex *ComplexObject) *ComplexObject {
	if complex != nil {
		if complex.Value == "" {
			complex.Value = "{}"
		}
	}
	return complex
}
