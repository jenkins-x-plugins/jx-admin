package gitconfig

// NewBlank initializes a blank Context suitable for testing
func NewBlank() *BlankContext {
	return &BlankContext{}
}

// A Context implementation that queries the filesystem
type BlankContext struct {
	authToken string
	authLogin string
}

func (c *BlankContext) AuthToken() (string, error) {
	return c.authToken, nil
}

func (c *BlankContext) SetAuthToken(t string) {
	c.authToken = t
}

func (c *BlankContext) SetAuthLogin(login string) {
	c.authLogin = login
}

func (c *BlankContext) AuthLogin() (string, error) {
	return c.authLogin, nil
}
