package gitconfig

// NewBlank initializes a blank Context suitable for testing
func NewBlank() *blankContext {
	return &blankContext{}
}

// A Context implementation that queries the filesystem
type blankContext struct {
	authToken string
	authLogin string
}

func (c *blankContext) AuthToken() (string, error) {
	return c.authToken, nil
}

func (c *blankContext) SetAuthToken(t string) {
	c.authToken = t
}

func (c *blankContext) SetAuthLogin(login string) {
	c.authLogin = login
}

func (c *blankContext) AuthLogin() (string, error) {
	return c.authLogin, nil
}
