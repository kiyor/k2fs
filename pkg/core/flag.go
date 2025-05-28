package core

// FlagDf holds the values for the -df command-line flag.
// It will be populated by Cobra in cmd/k2fs/main.go.
var FlagDf []string

// The flagSliceString type and its methods (String, Set) were specific
// to the standard library's 'flag' package. Cobra's StringSliceVar
// works directly with []string, so this custom type is likely no longer needed
// for flag parsing itself. If DiskSize or other functions specifically
// require this type, it might need to be kept or adapted.
// For now, it's removed to simplify and align with Cobra's direct use of []string.

// type flagSliceString []string

// func (i *flagSliceString) String() string {
// 	return ""
// }

// func (i *flagSliceString) Set(value string) error {
// 	*i = append(*i, value)
// 	return nil
// }
