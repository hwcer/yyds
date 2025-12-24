package channel

import "strings"

func Name(name, value string) string {
	return strings.Join([]string{name, value}, ".")
}
func Split(id string) (name, value string) {
	arr := strings.Split(id, ".")
	name = arr[0]
	if len(arr) > 1 {
		value = arr[1]
	}
	return
}
