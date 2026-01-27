package e2b

// Context represents an execution context for code.
// Contexts maintain isolated state for code execution.
type Context struct {
	// ID is the unique identifier for the context.
	ID string `json:"id"`

	// Language is the programming language of the context.
	Language string `json:"language"`

	// CWD is the current working directory of the context.
	CWD string `json:"cwd"`
}

// contextResponse is used for JSON unmarshaling from API responses.
type contextResponse struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	CWD      string `json:"cwd"`
}

// toContext converts a contextResponse to a Context.
func (c *contextResponse) toContext() *Context {
	return &Context{
		ID:       c.ID,
		Language: c.Language,
		CWD:      c.CWD,
	}
}
