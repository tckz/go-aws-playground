package playground

import "fmt"

type StringsFlag []string

func (f *StringsFlag) String() string {
	if f == nil {
		return "[]"
	}
	return fmt.Sprintf("%v", *f)
}

func (f *StringsFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}
