package builtin

func paramDefault(params map[string]string, key, def string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return def
}

func paramFirst(params map[string]string, def string, keys ...string) string {
	for _, key := range keys {
		if v, ok := params[key]; ok && v != "" {
			return v
		}
	}
	return def
}
