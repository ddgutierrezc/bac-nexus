package remote

// LiveValidationEnabled accepts only the explicit approved opt-in value. Its
// caller supplies environment lookup so absent configuration never infers a
// live IBM i target or opens a connection.
func LiveValidationEnabled(lookup func(string) (string, bool), key string) bool {
	if lookup == nil || key == "" {
		return false
	}
	value, present := lookup(key)
	return present && value == "1"
}
