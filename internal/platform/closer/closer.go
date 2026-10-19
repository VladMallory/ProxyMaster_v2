package closer

import (
	"errors"
	"io"
)

func HelpClose(body io.Closer, err *error) {
	if cerr := body.Close(); cerr != nil {
		*err = errors.Join(*err, cerr)
	}
}
