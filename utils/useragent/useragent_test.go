package useragent

import (
	"fmt"
	"testing"
)

func TestGet(t *testing.T) {
	number := hostnameToNumber()
	fmt.Println(number)
	got := Get()
	fmt.Println(got)
}
