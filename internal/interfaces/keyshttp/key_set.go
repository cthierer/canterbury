package keyshttp

type key struct {
	Kty string `json:"kty"`
	Use string `json:"use,omitempty"`
	Kid string `json:"kid,omitempty"`
	Alg string `json:"alg,omitempty"`

	// OKP
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
}

type keySet struct {
	Keys []key `json:"keys"`
}
