package main

type flagSliceString []string

func (i *flagSliceString) String() string {
	return ""
}

func (i *flagSliceString) Set(value string) error {
	*i = append(*i, value)
	return nil
}
