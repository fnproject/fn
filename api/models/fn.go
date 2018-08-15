package models

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/fnproject/fn/api/common"
)

var (
	// these are vars so that they can be configured. these apply
	// across function & trigger (resource config)

	MaxMemory      uint64 = 8 * 1024 // 8GB
	MaxTimeout     int32  = 300      // 5m
	MaxIdleTimeout int32  = 3600     // 1h

	ErrFnsIDMismatch = err{
		code:  http.StatusBadRequest,
		error: errors.New("Fn ID in path does not match that in body"),
	}
	ErrFnsIDProvided = err{
		code:  http.StatusBadRequest,
		error: errors.New("ID cannot be provided for Fn creation"),
	}
	ErrFnsMissingID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn ID"),
	}
	ErrFnsMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn name"),
	}
	ErrFnsInvalidName = err{
		code:  http.StatusBadRequest,
		error: errors.New("name must be a valid string"),
	}
	ErrFnsTooLongName = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Fn name must be %v characters or less", maxFnName),
	}
	ErrFnsMissingAppID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing AppID on Fn"),
	}
	ErrFnsMissingImage = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing image on Fn"),
	}
	ErrFnsInvalidFormat = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid format on Fn"),
	}
	ErrFnsInvalidTimeout = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("timeout value is out of range, must be between 0 and %d", MaxTimeout),
	}
	ErrFnsInvalidIdleTimeout = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("idle_timeout value is out of range, must be between 0 and %d", MaxIdleTimeout),
	}
	ErrFnsNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Fn not found"),
	}
	ErrFnsExists = err{
		code:  http.StatusConflict,
		error: errors.New("Fn with specified name already exists"),
	}
)

// FnInvokeEndpointAnnotation is the annotation that exposes the fn invoke endpoint For want of a better place to put this it's here
const FnInvokeEndpointAnnotation = "fnproject.io/fn/invokeEndpoint"

// Fn contains information about a function configuration.
type Fn struct {
	// ID is the generated resource id.
	ID string `json:"id" db:"id"`
	// Name is a user provided name for this fn.
	Name string `json:"name" db:"name"`
	// AppID is the name of the app this fn belongs to.
	AppID string `json:"app_id" db:"app_id"`
	// Image is the fully qualified container registry address to execute.
	// examples: hub.docker.io/me/myfunc, me/myfunc, me/func:0.0.1
	Image string `json:"image" db:"image"`
	// ResourceConfig specifies resource constraints.
	ResourceConfig // embed (TODO or not?)
	// Config is the configuration passed to a function at execution time.
	Config Config `json:"config" db:"config"`
	// Annotations allow additional configuration of a function, these are not passed to the function.
	Annotations Annotations `json:"annotations,omitempty" db:"annotations"`
	// CreatedAt is the UTC timestamp when this function was created.
	CreatedAt common.DateTime `json:"created_at,omitempty" db:"created_at"`
	// UpdatedAt is the UTC timestamp of the last time this func was modified.
	UpdatedAt common.DateTime `json:"updated_at,omitempty" db:"updated_at"`

	// TODO wish to kill but not yet ?
	// Format is the container protocol the function will accept,
	// may be one of: json | http | cloudevent | default
	Format string `json:"format" db:"format"`
}

// ResourceConfig specified resource constraints imposed on a function execution.
type ResourceConfig struct {
	// Memory is the amount of memory allotted, in MB.
	Memory uint64 `json:"memory,omitempty" db:"memory"`
	// Timeout is the max execution time for a function, in seconds.
	// TODO this should probably be milliseconds?
	Timeout int32 `json:"timeout,omitempty" db:"timeout"`
	// IdleTimeout is the
	// TODO this should probably be milliseconds
	IdleTimeout int32 `json:"idle_timeout,omitempty" db:"idle_timeout"`
}

// SetCreated sets zeroed field to defaults.
func (f *Fn) SetDefaults() {

	if f.Memory == 0 {
		f.Memory = DefaultMemory
	}

	if f.Format == "" {
		f.Format = FormatDefault
	}

	if f.Config == nil {
		// keeps the json from being nil
		f.Config = map[string]string{}
	}

	if f.Timeout == 0 {
		f.Timeout = DefaultTimeout
	}

	if f.IdleTimeout == 0 {
		f.IdleTimeout = DefaultIdleTimeout
	}

	if time.Time(f.CreatedAt).IsZero() {
		f.CreatedAt = common.DateTime(time.Now())
	}

	if time.Time(f.UpdatedAt).IsZero() {
		f.UpdatedAt = common.DateTime(time.Now())
	}
}

// Validate validates all field values, returning the first error, if any.
func (f *Fn) Validate() error {

	if f.Name == "" {
		return ErrFnsMissingName
	}
	if len(f.Name) > maxFnName {
		return ErrFnsTooLongName
	}

	if url.PathEscape(f.Name) != f.Name {
		return ErrFnsInvalidName
	}

	if f.AppID == "" {
		return ErrFnsMissingAppID
	}

	if f.Image == "" {
		return ErrFnsMissingImage
	}

	switch f.Format {
	case FormatDefault, FormatHTTP, FormatJSON, FormatCloudEvent:
	default:
		return ErrFnsInvalidFormat
	}

	if f.Timeout <= 0 || f.Timeout > MaxTimeout {
		return ErrFnsInvalidTimeout
	}

	if f.IdleTimeout <= 0 || f.IdleTimeout > MaxIdleTimeout {
		return ErrFnsInvalidIdleTimeout
	}

	if f.Memory < 1 || f.Memory > MaxMemory {
		return ErrInvalidMemory
	}

	return f.Annotations.Validate()
}

func (f *Fn) Clone() *Fn {
	clone := new(Fn)
	*clone = *f // shallow copy

	// now deep copy the maps
	if f.Config != nil {
		clone.Config = make(Config, len(f.Config))
		for k, v := range f.Config {
			clone.Config[k] = v
		}
	}
	if f.Annotations != nil {
		clone.Annotations = make(Annotations, len(f.Annotations))
		for k, v := range f.Annotations {
			// TODO technically, we need to deep copy the bytes
			clone.Annotations[k] = v
		}
	}
	return clone
}

func (f1 *Fn) Equals(f2 *Fn) bool {
	// start off equal, check equivalence of each field.
	// the RHS of && won't eval if eq==false so config/headers checking is lazy

	eq := true
	eq = eq && f1.ID == f2.ID
	eq = eq && f1.Name == f2.Name
	eq = eq && f1.AppID == f2.AppID
	eq = eq && f1.Image == f2.Image
	eq = eq && f1.Memory == f2.Memory
	eq = eq && f1.Format == f2.Format
	eq = eq && f1.Timeout == f2.Timeout
	eq = eq && f1.IdleTimeout == f2.IdleTimeout
	eq = eq && f1.Config.Equals(f2.Config)
	eq = eq && f1.Annotations.Equals(f2.Annotations)
	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(f1.CreatedAt).Equal(time.Time(f2.CreatedAt))
	//eq = eq && time.Time(f2.UpdatedAt).Equal(time.Time(f2.UpdatedAt))
	return eq
}

func (f1 *Fn) EqualsWithAnnotationSubset(f2 *Fn) bool {
	// start off equal, check equivalence of each field.
	// the RHS of && won't eval if eq==false so config/headers checking is lazy

	eq := true
	eq = eq && f1.ID == f2.ID
	eq = eq && f1.Name == f2.Name
	eq = eq && f1.AppID == f2.AppID
	eq = eq && f1.Image == f2.Image
	eq = eq && f1.Memory == f2.Memory
	eq = eq && f1.Format == f2.Format
	eq = eq && f1.Timeout == f2.Timeout
	eq = eq && f1.IdleTimeout == f2.IdleTimeout
	eq = eq && f1.Config.Equals(f2.Config)
	eq = eq && f1.Annotations.Subset(f2.Annotations)
	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(f1.CreatedAt).Equal(time.Time(f2.CreatedAt))
	//eq = eq && time.Time(f2.UpdatedAt).Equal(time.Time(f2.UpdatedAt))
	return eq
}

// Update updates fields in f with non-zero field values from new, and sets
// updated_at if any of the fields change. 0-length slice Header values, and
// empty-string Config values trigger removal of map entry.
func (f *Fn) Update(patch *Fn) {
	original := f.Clone()

	if patch.Image != "" {
		f.Image = patch.Image
	}
	if patch.Memory != 0 {
		f.Memory = patch.Memory
	}

	if patch.Timeout != 0 {
		f.Timeout = patch.Timeout
	}
	if patch.IdleTimeout != 0 {
		f.IdleTimeout = patch.IdleTimeout
	}
	if patch.Format != "" {
		f.Format = patch.Format
	}
	if patch.Config != nil {
		if f.Config == nil {
			f.Config = make(Config)
		}
		for k, v := range patch.Config {
			if v == "" {
				delete(f.Config, k)
			} else {
				f.Config[k] = v
			}
		}
	}

	f.Annotations = f.Annotations.MergeChange(patch.Annotations)

	if !f.Equals(original) {
		f.UpdatedAt = common.DateTime(time.Now())
	}
}

type FnFilter struct {
	AppID   string // this is exact match
	Name    string //exact match
	Cursor  string
	PerPage int
}

type FnList struct {
	NextCursor string `json:"next_cursor,omitempty"`
	Items      []*Fn  `json:"items"`
}
