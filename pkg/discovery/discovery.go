package discovery

// Discovery abstracts how seed nodes are provided.
// Future implementations may include DNS, file, or dynamic sources.
type Discovery interface {
    Seeds() []string
}

