//go:build !cgo

package filterusername

// cgoSource is the CGO-backed user database lookup. In a non-CGO build it is nil,
// which makes the auto source fall back to the pure Go getent lookup.
var cgoSource func() ([]string, error) = nil
