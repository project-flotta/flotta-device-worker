package registration

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/pelletier/go-toml"
)

// TO BE REMOVED, we should export these functions to his own package
// Link to the source-code:
// https://github.com/RedHatInsights/yggdrasil/blob/cdde836c519ac72302714b5f7ae90e45de7c79a7/cmd/yggd/tags.go
type errorTag struct {
	v interface{}
}

func (e *errorTag) Error() string {
	return fmt.Sprintf("cannot parse '%T' as string", e.v)
}

func (e *errorTag) Is(o error) bool {
	return reflect.TypeOf(e) == reflect.TypeOf(o)
}

// readTags reads from its input, unmarshalling the TOML-encoded value to a map.
// It then parses the map values into a map of string values.
func readTags(in io.Reader) (map[string]string, error) {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("cannot read input: %w", err)
	}

	var rawTags map[string]interface{}
	if err := toml.Unmarshal(data, &rawTags); err != nil {
		return nil, fmt.Errorf("cannot parse TOML: %w", err)
	}

	tags := make(map[string]string)
	for k, v := range rawTags {
		switch v := v.(type) {
		case string:
			tags[k] = v
		case int, int8, int16, int32, int64:
			tags[k] = strconv.FormatInt(v.(int64), 10)
		case float32, float64:
			tags[k] = strconv.FormatFloat(v.(float64), 'g', -1, 64)
		case bool:
			tags[k] = strconv.FormatBool(v)
		case toml.LocalDate, toml.LocalTime, toml.LocalDateTime:
			tags[k] = v.(fmt.Stringer).String()
		case time.Time:
			tags[k] = v.Format(time.RFC3339)
		default:
			return nil, &errorTag{v}
		}
	}

	return tags, nil
}

// readTagsFile reads tag data from file.
func readTagsFile(file string) (map[string]string, error) {
	f, err := os.Open(filepath.Clean(file))
	defer func() { _ = f.Close() }()
	if err != nil {
		return nil, fmt.Errorf("cannot open '%v' for reading: %w", file, err)
	}

	tags, err := readTags(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read tags file: %w", err)
	}
	return tags, nil
}
