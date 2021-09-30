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

// Error error
//
// swagger:model error
type Error struct {

	// Globally unique code of the error, composed of the unique identifier of the API and the numeric identifier of the error. For example, if the numeric identifier of the error is 93 and the identifier of the API is assisted_install then the code will be ASSISTED-INSTALL-93.
	// Required: true
	Code *string `json:"code"`

	// Self link.
	// Required: true
	Href *string `json:"href"`

	// Numeric identifier of the error.
	// Required: true
	// Maximum: 504
	// Minimum: 400
	ID *int32 `json:"id"`

	// Indicates the type of this object. Will always be 'Error'.
	// Required: true
	// Enum: [Error]
	Kind *string `json:"kind"`

	// Human-readable description of the error.
	// Required: true
	Reason *string `json:"reason"`
}

// Validate validates this error
func (m *Error) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateCode(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateHref(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateID(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateKind(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateReason(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *Error) validateCode(formats strfmt.Registry) error {

	if err := validate.Required("code", "body", m.Code); err != nil {
		return err
	}

	return nil
}

func (m *Error) validateHref(formats strfmt.Registry) error {

	if err := validate.Required("href", "body", m.Href); err != nil {
		return err
	}

	return nil
}

func (m *Error) validateID(formats strfmt.Registry) error {

	if err := validate.Required("id", "body", m.ID); err != nil {
		return err
	}

	if err := validate.MinimumInt("id", "body", int64(*m.ID), 400, false); err != nil {
		return err
	}

	if err := validate.MaximumInt("id", "body", int64(*m.ID), 504, false); err != nil {
		return err
	}

	return nil
}

var errorTypeKindPropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["Error"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		errorTypeKindPropEnum = append(errorTypeKindPropEnum, v)
	}
}

const (

	// ErrorKindError captures enum value "Error"
	ErrorKindError string = "Error"
)

// prop value enum
func (m *Error) validateKindEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, errorTypeKindPropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *Error) validateKind(formats strfmt.Registry) error {

	if err := validate.Required("kind", "body", m.Kind); err != nil {
		return err
	}

	// value enum
	if err := m.validateKindEnum("kind", "body", *m.Kind); err != nil {
		return err
	}

	return nil
}

func (m *Error) validateReason(formats strfmt.Registry) error {

	if err := validate.Required("reason", "body", m.Reason); err != nil {
		return err
	}

	return nil
}

// MarshalBinary interface implementation
func (m *Error) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *Error) UnmarshalBinary(b []byte) error {
	var res Error
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
