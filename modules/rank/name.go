package rank

import (
	"fmt"
	"reflect"
	"strconv"
)

// ParseName 将排行榜名称转换成确定的字符串,仅支持底层类型为字符串和数字的值
//
// 允许自定义类型,如 protobuf 生成的枚举(底层 int32),此时取其数值
//
// 用于生成 REDIS KEY 以及 Master 中的索引,必须保证同一个 name 每次转换结果一致
func ParseName(name any) (string, error) {
	switch v := name.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("排行榜名称不能为空")
		}
		return v, nil
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return parseNameReflect(name)
	}
}

// parseNameReflect 处理自定义类型(如 protobuf 枚举),按底层类型取值
func parseNameReflect(name any) (string, error) {
	v := reflect.ValueOf(name)
	switch v.Kind() {
	case reflect.String:
		if v.String() == "" {
			return "", fmt.Errorf("排行榜名称不能为空")
		}
		return v.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32), nil
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("排行榜名称仅支持字符串和数字:%v(%T)", name, name)
	}
}
