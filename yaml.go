package playground

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/goccy/go-yaml"
)

func OutputAsYAML(out any, w io.Writer) error {
	b, err := marshalYAML(out)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "---"); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, string(b)); err != nil {
		return err
	}
	return nil
}

func marshalYAML(s any) ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	b, err = yaml.JSONToYAML(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
