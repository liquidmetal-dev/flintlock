package ptr

func Bool(val bool) *bool {
	return &val
}

func String(val string) *string {
	return &val
}
