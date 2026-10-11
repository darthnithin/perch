package launch

// FieldError identifies the input to correct without making UI callers parse
// human-readable error text. The same error remains useful to CLI callers.
type FieldError struct {
	Field string
	Err   error
}

func (e *FieldError) Error() string { return e.Err.Error() }
func (e *FieldError) Unwrap() error { return e.Err }

func invalidField(field string, err error) error {
	return &FieldError{Field: field, Err: err}
}
