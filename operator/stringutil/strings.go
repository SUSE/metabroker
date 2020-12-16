package stringutil

// Contains returns whether `slice` contains the string `s` or not.
func Contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Remove returns a new slice with the first match of `s` removed from `slice`.
func Remove(slice []string, s string) []string {
	for i, item := range slice {
		if item == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
