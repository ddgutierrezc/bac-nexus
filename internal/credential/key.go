package credential

// KeyForProfile derives the stable native-keyring account without including
// endpoint, username, or secret material.
func KeyForProfile(profile string) (string, error) { return nativeAccount(profile) }
