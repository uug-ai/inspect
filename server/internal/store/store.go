// Package store persists attachment media somewhere an issue can link to.
package store

import "context"

// Store saves one file and returns a URL the issue body can reference.
// name is a slash-separated relative path chosen by the caller (for example
// "inspect/2026/11/12/<id>/screenshot-1.png"); implementations must not
// interpret it beyond that. Further backends (S3, MinIO, ...) only need to
// implement this interface.
type Store interface {
	Put(ctx context.Context, name, contentType string, data []byte) (url string, err error)
}
