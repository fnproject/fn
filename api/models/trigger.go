package models

import (
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode"

	"github.com/fnproject/fn/api/common"
)

// For want of a better place to put this it's here
const TriggerHTTPEndpointAnnotation = "fnproject.io/trigger/httpEndpoint"

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

const TriggerTypeHTTP = "http"

var triggerTypes = []string{TriggerTypeHTTP}

func ValidTriggerTypes() []string {
	return triggerTypes
}

func ValidTriggerType(a string) bool {
	for _, b := range triggerTypes {
		if b == a {
			return true
		}
	}
	return false
}

var (
	ErrTriggerIDProvided = err{
		code:  http.StatusBadRequest,
		error: errors.New("ID cannot be provided for Trigger creation"),
	}
	ErrTriggerIDMismatch = err{
		code:  http.StatusBadRequest,
		error: errors.New("ID in path does not match ID in body"),
	}
	ErrTriggerMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing name on Trigger")}
	ErrTriggerTooLongName = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Trigger name must be %v characters or less", MaxTriggerName)}
	ErrTriggerInvalidName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid name for Trigger")}
	ErrTriggerMissingAppID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing App ID on Trigger")}
	ErrTriggerMissingFnID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn ID on Trigger")}
	ErrTriggerFnIDNotSameApp = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid Fn ID - not owned by specified app")}
	ErrTriggerTypeUnknown = err{
		code:  http.StatusBadRequest,
		error: errors.New("Trigger Type Not Supported")}
	ErrTriggerMissingSource = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Trigger Source")}
	ErrTriggerNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Trigger not found")}
	ErrTriggerExists = err{
		code:  http.StatusConflict,
		error: errors.New("Trigger already exists")}
)

func (t *Trigger) Validate() error {
	if t.Name == "" {
		return ErrTriggerMissingName
	}

	if t.AppID == "" {
		return ErrTriggerMissingAppID
	}

	if len(t.Name) > MaxTriggerName {
		return ErrTriggerTooLongName
	}
	for _, c := range t.Name {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c) || c == '_' || c == '-') {
			return ErrTriggerInvalidName
		}
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

	err := t.Annotations.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (t *Trigger) Clone() *Trigger {
	clone := new(Trigger)
	*clone = *t // shallow copy

	if t.Annotations != nil {
		clone.Annotations = make(Annotations, len(t.Annotations))
		for k, v := range t.Annotations {
			// TODO technically, we need to deep copy the bytes
			clone.Annotations[k] = v
		}
	}
	return clone
}

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

type TriggerFilter struct {
	AppID string // this is exact match
	FnID  string // this is exact match
	Name  string // exact match

	Cursor  string
	PerPage int
}

type TriggerList struct {
	NextCursor string     `json:"next_cursor,omitempty"`
	Items      []*Trigger `json:"items"`
}
