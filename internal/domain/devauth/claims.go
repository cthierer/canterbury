package devauth

// Claims describes identity claims requested for a development token.
type Claims struct {
	Subject   string
	Audiences []string
}
