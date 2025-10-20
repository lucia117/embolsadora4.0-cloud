package security

// TODO: Define API Keys validation and middleware wiring.

// APIKeyLookup resolves API Keys to tenant identity and scopes. Do not store raw keys.
type APIKeyLookup interface {
    Lookup(key string) (tenantID string, apiKeyID string, scopes []string, ok bool)
}

// StubAPIKeyLookup returns a no-op lookup placeholder.
func StubAPIKeyLookup() APIKeyLookup { return stubAPIKeyLookup{} }

type stubAPIKeyLookup struct{}

func (stubAPIKeyLookup) Lookup(key string) (string, string, []string, bool) {
    // TODO: implement lookup against hashed keys storage.
    return "", "", nil, false
}
