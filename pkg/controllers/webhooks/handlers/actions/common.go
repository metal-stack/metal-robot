package actions

type ContentReader func(file string) ([]byte, error)
type ContentWriter func(file string, content []byte) error
