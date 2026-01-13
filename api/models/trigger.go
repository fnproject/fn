package models

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/fnproject/fn/api/common"
)

// TriggerHTTPEndpointAnnotation is the annotation that exposes the HTTP trigger endpoint For want of a better place to put this it's here
const TriggerHTTPEndpointAnnotation = "fnproject.io/trigger/httpEndpoint"

// Trigger represents a binding between a Function and an external event source
type Trigger struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	AppID       string          `json:"app_id" db:"app_id"`
	FnID        string          `json:"fn_id" db:"fn_id"`
	CreatedAt   common.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   common.DateTime `json:"updated_at,omitempty" db:"updated_at"`
	Type        string          `json:"type" db:"type"`
	Source      string          `json:"source" db:"source"`
	Annotations Annotations     `json:"annotations,omitempty" db:"annotations"`
}

// Equals compares two triggers for semantic equality  it ignores timestamp fields but includes annotations
func (t *Trigger) Equals(t2 *Trigger) bool {
	eq := true
	eq = eq && t.ID == t2.ID
	eq = eq && t.Name == t2.Name
	eq = eq && t.AppID == t2.AppID
	eq = eq && t.FnID == t2.FnID

	eq = eq && t.Type == t2.Type
	eq = eq && t.Source == t2.Source
	eq = eq && t.Annotations.Equals(t2.Annotations)

	return eq
}

// EqualsWithAnnotationSubset is equivalent to Equals except it accepts cases where t's annotations are strict subset of t2
func (t *Trigger) EqualsWithAnnotationSubset(t2 *Trigger) bool {
	eq := true
	eq = eq && t.ID == t2.ID
	eq = eq && t.Name == t2.Name
	eq = eq && t.AppID == t2.AppID
	eq = eq && t.FnID == t2.FnID

	eq = eq && t.Type == t2.Type
	eq = eq && t.Source == t2.Source
	eq = eq && t.Annotations.Subset(t2.Annotations)

	return eq
}

// TriggerTypeHTTP represents an HTTP trigger
const TriggerTypeHTTP = "http"

var triggerTypes = []string{TriggerTypeHTTP}

// ValidTriggerTypes lists the supported trigger types in this service
func ValidTriggerTypes() []string {
	return triggerTypes
}

// ValidTriggerType checks that a given trigger type is valid on this service
func ValidTriggerType(a string) bool {
	for _, b := range triggerTypes {
		if b == a {
			return true
		}
	}
	return false
}

var (
	//ErrTriggerIDProvided indicates that a trigger ID was specified when it shouldn't have been
	ErrTriggerIDProvided = err{
		code:  http.StatusBadRequest,
		error: errors.New("ID cannot be provided for Trigger creation"),
	}
	//ErrTriggerIDMismatch indicates an ID was provided that did not match the ID of the corresponding operation/call
	ErrTriggerIDMismatch = err{
		code:  http.StatusBadRequest,
		error: errors.New("ID in path does not match ID in body"),
	}
	//ErrTriggerMissingName - name not specified on a trigger object
	ErrTriggerMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing name on Trigger")}
	//ErrTriggerTooLongName - name exceeds maximum permitted name
	ErrTriggerTooLongName = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Trigger name must be %v characters or less", MaxLengthTriggerName)}
	//ErrTriggerInvalidName - name does not comply with naming spec
	ErrTriggerInvalidName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid name for Trigger")}
	//ErrTriggerMissingAppID - no API id specified on trigger creation
	ErrTriggerMissingAppID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing App ID on Trigger")}
	//ErrTriggerMissingFnID - no FNID specified on trigger creation
	ErrTriggerMissingFnID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn ID on Trigger")}
	//ErrTriggerFnIDNotSameApp - specified Fn does not belong to the same app as the provided AppID
	ErrTriggerFnIDNotSameApp = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid Fn ID - not owned by specified app")}
	//ErrTriggerTypeUnknown - unsupported trigger type
	ErrTriggerTypeUnknown = err{
		code:  http.StatusBadRequest,
		error: errors.New("Trigger Type Not Supported")}
	//ErrTriggerMissingSource - no source spceified for trigger
	ErrTriggerMissingSource = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Trigger Source")}
	//ErrTriggerMissingSourcePrefix - source does not have a / prefix
	ErrTriggerMissingSourcePrefix = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Trigger Source Prefix '/'")}
	//ErrTriggerNotFound - trigger not found
	ErrTriggerNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Trigger not found")}
	//ErrTriggerExists - a trigger with the specified name already exists
	ErrTriggerExists = err{
		code:  http.StatusConflict,
		error: errors.New("Trigger already exists")}
	//ErrTriggerSourceExists - another trigger on the same app has the same source and type
	ErrTriggerSourceExists = err{
		code:  http.StatusConflict,
		error: errors.New("Trigger with the same type and source exists on this app")}
)

// Validate checks that trigger has valid data for inserting into a store
func (t *Trigger) Validate() error {
	if t.AppID == "" {
		return ErrTriggerMissingAppID
	}

	if err := t.ValidateName(); err != nil {
		return err
	}

	if t.FnID == "" {
		return ErrTriggerMissingFnID
	}

	if !ValidTriggerType(t.Type) {
		return ErrTriggerTypeUnknown
	}

	if t.Source == "" {
		return ErrTriggerMissingSource
	}

	if !strings.HasPrefix(t.Source, "/") {
		return ErrTriggerMissingSourcePrefix
	}

	err := t.Annotations.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (t *Trigger) ValidateName() error {
	if t.Name == "" {
		return ErrTriggerMissingName
	}

	if len(t.Name) > MaxLengthTriggerName {
		return ErrTriggerTooLongName
	}

	for _, c := range t.Name {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c) || c == '_' || c == '-') {
			return ErrTriggerInvalidName
		}
	}

	return nil
}

// Clone creates a deep copy of a trigger
func (t *Trigger) Clone() *Trigger {
	clone := new(Trigger)
	*clone = *t // shallow copy
	// annotations are immutable via their interface so can be shallow copied
	return clone
}

// Update applies a change to a trigger
func (t *Trigger) Update(patch *Trigger) {

	original := t.Clone()
	if patch.AppID != "" {
		t.AppID = patch.AppID
	}

	if patch.FnID != "" {
		t.FnID = patch.FnID
	}

	if patch.Name != "" {
		t.Name = patch.Name
	}

	if patch.Source != "" {
		t.Source = patch.Source
	}

	t.Annotations = t.Annotations.MergeChange(patch.Annotations)

	if !t.Equals(original) {
		t.UpdatedAt = common.DateTime(time.Now())
	}
}

// TriggerFilter is a search criteria on triggers
type TriggerFilter struct {
	//AppID searches for triggers in APP - mandatory
	AppID string // this is exact match mandatory
	//FNID searches for triggers belonging to a specific function
	FnID string // this is exact match
	//Name is the name of the trigger
	Name string // exact match

	Cursor  string
	PerPage int
}

// TriggerList is a container of triggers returned by search, optionally indicating the next page cursor
type TriggerList struct {
	NextCursor string     `json:"next_cursor,omitempty"`
	Items      []*Trigger `json:"items"`
}
