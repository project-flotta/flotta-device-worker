// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// WorkloadStatus workload status
//
// swagger:model workload-status
type WorkloadStatus struct {

	// last data upload
	// Format: date-time
	LastDataUpload strfmt.DateTime `json:"last_data_upload,omitempty"`

	// name
	Name string `json:"name,omitempty"`

	// status
	// Enum: [deploying running crashed stopped]
	Status string `json:"status,omitempty"`
}

// Validate validates this workload status
func (m *WorkloadStatus) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateLastDataUpload(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateStatus(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *WorkloadStatus) validateLastDataUpload(formats strfmt.Registry) error {

	if swag.IsZero(m.LastDataUpload) { // not required
		return nil
	}

	if err := validate.FormatOf("last_data_upload", "body", "date-time", m.LastDataUpload.String(), formats); err != nil {
		return err
	}

	return nil
}

var workloadStatusTypeStatusPropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["deploying","running","crashed","stopped"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		workloadStatusTypeStatusPropEnum = append(workloadStatusTypeStatusPropEnum, v)
	}
}

const (

	// WorkloadStatusStatusDeploying captures enum value "deploying"
	WorkloadStatusStatusDeploying string = "deploying"

	// WorkloadStatusStatusRunning captures enum value "running"
	WorkloadStatusStatusRunning string = "running"

	// WorkloadStatusStatusCrashed captures enum value "crashed"
	WorkloadStatusStatusCrashed string = "crashed"

	// WorkloadStatusStatusStopped captures enum value "stopped"
	WorkloadStatusStatusStopped string = "stopped"
)

// prop value enum
func (m *WorkloadStatus) validateStatusEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, workloadStatusTypeStatusPropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *WorkloadStatus) validateStatus(formats strfmt.Registry) error {

	if swag.IsZero(m.Status) { // not required
		return nil
	}

	// value enum
	if err := m.validateStatusEnum("status", "body", m.Status); err != nil {
		return err
	}

	return nil
}

// MarshalBinary interface implementation
func (m *WorkloadStatus) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *WorkloadStatus) UnmarshalBinary(b []byte) error {
	var res WorkloadStatus
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
