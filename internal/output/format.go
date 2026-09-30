package output

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func PrintJSON(data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// FormatValue prints a decoded JSON value, numbers without exponents
// (1073741824, not 1.073741824e+09).
func FormatValue(v interface{}) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}
