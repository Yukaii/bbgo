package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/types"
)

type Instance interface {
	ID() string
	InstanceID() string
}

type InstancePropertyIndex struct {
	ID         string
	InstanceID string
	Property   string
}

type StateRecorder struct {
	outputDirectory string
	strategies      []Instance
	files           map[interface{}]*os.File
	writers         map[types.CsvFormatter]*csv.Writer
	manifests       Manifests
}

func NewStateRecorder(outputDir string) *StateRecorder {
	return &StateRecorder{
		outputDirectory: outputDir,
		files:           make(map[interface{}]*os.File),
		writers:         make(map[types.CsvFormatter]*csv.Writer),
		manifests:       make(Manifests),
	}
}

func (r *StateRecorder) Snapshot() (int, error) {
	var c int
	for obj, writer := range r.writers {
		records := obj.CsvRecords()
		for _, record := range records {
			if err := writer.Write(record); err != nil {
				return c, err
			}
			c++
		}

		writer.Flush()
	}
	return c, nil
}

func (r *StateRecorder) Scan(instance Instance) error {
	r.strategies = append(r.strategies, instance)

	rt := reflect.TypeOf(instance)
	rv := reflect.ValueOf(instance)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		rv = rv.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return fmt.Errorf("given object is not a struct: %+v", rt)
	}

	for i := 0; i < rt.NumField(); i++ {
		structField := rt.Field(i)
		if !structField.IsExported() {
			continue
		}

		obj := rv.Field(i).Interface()
		switch o := obj.(type) {

		case types.CsvFormatter: // interface type
			typeName := strings.ToLower(structField.Type.Elem().Name())
			if typeName == "" {
				return fmt.Errorf("%v is a non-defined type", structField.Type)
			}

			if err := r.newCsvWriter(o, instance, typeName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *StateRecorder) formatCsvFilename(instance Instance, objType string) string {
	return filepath.Join(r.outputDirectory, fmt.Sprintf("%s-%s.csv", instance.InstanceID(), objType))
}

func (r *StateRecorder) Manifests() Manifests {
	return r.manifests
}

func (r *StateRecorder) newCsvWriter(o types.CsvFormatter, instance Instance, typeName string) error {
	fn := r.formatCsvFilename(instance, typeName)
	f, err := os.Create(fn)
	if err != nil {
		return err
	}

	if _, exists := r.files[o]; exists {
		return fmt.Errorf("file of object %v already exists", o)
	}

	r.manifests[InstancePropertyIndex{
		ID:         instance.ID(),
		InstanceID: instance.InstanceID(),
		Property:   typeName,
	}] = fn

	r.files[o] = f

	w := csv.NewWriter(f)
	r.writers[o] = w

	return w.Write(o.CsvHeader())
}

func (r *StateRecorder) Close() error {
	var err error

	for _, f := range r.files {
		err2 := f.Close()
		if err2 != nil {
			err = multierr.Append(err, err2)
		}
	}

	return err
}
