package nodego

import (
	"io"
	"encoding/gob"
)


type gobEncoding struct {}

func (encoding *gobEncoding) Decode(reader io.Reader, e interface{}) error{
	dec :=  gob.NewDecoder(reader)

	return dec.Decode(e)
}

func (encoding *gobEncoding)  Encode(writer io.Writer, e interface{}) error{
	enc :=  gob.NewEncoder(writer)
	return enc.Encode(e)
}