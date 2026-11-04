package usecase

// Keyring is where the secrets of the accounts are kept: the OS keychain, or
// a stand-in in a test. It has the methods of auth.Keyring.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}
