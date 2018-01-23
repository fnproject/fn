// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

// TODO: define this as package validate/internal
// This must be done while keeping CI intact with all tests and test coverage

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
)

// Helpers available at the package level
var (
	pathHelp     *pathHelper
	valueHelp    *valueHelper
	errorHelp    *errorHelper
	paramHelp    *paramHelper
	responseHelp *responseHelper
)

type errorHelper struct {
	// A collection of unexported helpers for error construction
}

func (h *errorHelper) sErr(err errors.Error) *Result {
	// Builds a Result from standard errors.Error
	return &Result{Errors: []error{err}}
}

func (h *errorHelper) addPointerError(res *Result, err error, ref string, fromPath string) *Result {
	// Provides more context on error messages
	// reported by the jsoinpointer package by altering the passed Result
	if err != nil {
		res.AddErrors(cannotResolveRefMsg(fromPath, ref, err))
	}
	return res
}

type pathHelper struct {
	// A collection of unexported helpers for path validation
}

func (h *pathHelper) stripParametersInPath(path string) string {
	// Returns a path stripped from all path parameters, with multiple or trailing slashes removed.
	//
	// Stripping is performed on a slash-separated basis, e.g '/a{/b}' remains a{/b} and not /a.
	//  - Trailing "/" make a difference, e.g. /a/ !~ /a (ex: canary/bitbucket.org/swagger.json)
	//  - presence or absence of a parameter makes a difference, e.g. /a/{log} !~ /a/ (ex: canary/kubernetes/swagger.json)

	// Regexp to extract parameters from path, with surrounding {}.
	// NOTE: important non-greedy modifier
	rexParsePathParam := mustCompileRegexp(`{[^{}]+?}`)
	strippedSegments := []string{}

	for _, segment := range strings.Split(path, "/") {
		strippedSegments = append(strippedSegments, rexParsePathParam.ReplaceAllString(segment, "X"))
	}
	return strings.Join(strippedSegments, "/")
}

func (h *pathHelper) extractPathParams(path string) (params []string) {
	// Extracts all params from a path, with surrounding "{}"
	rexParsePathParam := mustCompileRegexp(`{[^{}]+?}`)

	for _, segment := range strings.Split(path, "/") {
		for _, v := range rexParsePathParam.FindAllStringSubmatch(segment, -1) {
			params = append(params, v...)
		}
	}
	return
}

type valueHelper struct {
	// A collection of unexported helpers for value validation
}

func (h *valueHelper) asInt64(val interface{}) int64 {
	// Number conversion function for int64, without error checking
	// (implements an implicit type upgrade).
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return int64(v.Float())
	default:
		//panic("Non numeric value in asInt64()")
		return 0
	}
}

func (h *valueHelper) asUint64(val interface{}) uint64 {
	// Number conversion function for uint64, without error checking
	// (implements an implicit type upgrade).
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return uint64(v.Float())
	default:
		//panic("Non numeric value in asUint64()")
		return 0
	}
}

// Same for unsigned floats
func (h *valueHelper) asFloat64(val interface{}) float64 {
	// Number conversion function for float64, without error checking
	// (implements an implicit type upgrade).
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		//panic("Non numeric value in asFloat64()")
		return 0
	}
}

type paramHelper struct {
	// A collection of unexported helpers for parameters resolution
}

func (h *paramHelper) safeExpandedParamsFor(path, method, operationID string, res *Result, s *SpecValidator) (params []spec.Parameter) {
	for _, ppr := range s.analyzer.SafeParamsFor(method, path,
		func(p spec.Parameter, err error) bool {
			res.AddErrors(someParametersBrokenMsg(path, method, operationID))
			// original error from analyzer
			res.AddErrors(err)
			return true
		}) {
		pr, red := h.resolveParam(path, method, operationID, ppr, s)
		if red.HasErrors() { // Safeguard
			// NOTE: it looks like the new spec.Ref.GetPointer() method expands the full tree, so this code is no more reachable
			res.Merge(red)
			if red.HasErrors() && !s.Options.ContinueOnErrors {
				break
			}
			continue
		}
		params = append(params, pr)
	}
	return
}

func (h *paramHelper) resolveParam(path, method, operationID string, ppr spec.Parameter, s *SpecValidator) (spec.Parameter, *Result) {
	// Resolve references with any depth for parameter
	// NOTE: the only difference with what analysis does is in the "for": analysis SafeParamsFor() stops at first ref.
	res := new(Result)
	pr := ppr
	sw := s.spec.Spec()

	for pr.Ref.String() != "" {
		obj, _, err := pr.Ref.GetPointer().Get(sw)
		if err != nil { // Safeguard
			// NOTE: it looks like the new spec.Ref.GetPointer() method expands the full tree, so this code is no more reachable
			refPath := strings.Join([]string{"\"" + path + "\"", method}, ".")
			errorHelp.addPointerError(res, err, pr.Ref.String(), refPath)
			pr = spec.Parameter{}
		} else {
			if checkedObj, ok := h.checkedParamAssertion(obj, pr.Name, pr.In, operationID, res); ok {
				pr = checkedObj
			} else {
				pr = spec.Parameter{}
			}
		}
	}
	return pr, res
}

func (h *paramHelper) checkedParamAssertion(obj interface{}, path, in, operation string, res *Result) (spec.Parameter, bool) {
	// Secure parameter type assertion and try to explain failure
	if checkedObj, ok := obj.(spec.Parameter); ok {
		return checkedObj, true
	}
	// Try to explain why... best guess
	if _, ok := obj.(spec.Schema); ok {
		// Most likely, a $ref with a sibling is an unwanted situation: in itself this is a warning...
		res.AddWarnings(refShouldNotHaveSiblingsMsg(path, operation))
		// but we detect it because of the following error:
		// schema took over Parameter for an unexplained reason
		res.AddErrors(invalidParameterDefinitionAsSchemaMsg(path, in, operation))
	} else { // Safeguard
		// NOTE: the only known case for this error is $ref expansion replaced parameter by a Schema
		// Here, another structure replaced spec.Parameter. We should not be able to enter there. Croaks a generic error.
		res.AddErrors(invalidParameterDefinitionMsg(path, in, operation))
	}
	return spec.Parameter{}, false
}

type responseHelper struct {
	// A collection of unexported helpers for response resolution
}

func (r *responseHelper) expandResponseRef(response *spec.Response, path string, s *SpecValidator) (*spec.Response, *Result) {
	// Recursively follow possible $ref's on responses
	res := new(Result)
	for response.Ref.String() != "" {
		obj, _, err := response.Ref.GetPointer().Get(s.spec.Spec())
		if err != nil { // Safeguard
			// NOTE: with ref expansion in spec, this code is no more reachable
			errorHelp.addPointerError(res, err, response.Ref.String(), strings.Join([]string{"\"" + path + "\"", response.ResponseProps.Schema.ID}, "."))
			break
		}
		// Here we may expect type assertion to be guaranteed (not like in the Parameter case)
		nr := obj.(spec.Response)
		response = &nr
	}
	return response, res
}

func (r *responseHelper) responseMsgVariants(responseType string, responseCode int) (responseName, responseCodeAsStr string) {
	// Path variants for messages
	if responseType == "default" {
		responseCodeAsStr = "default"
		responseName = "default response"
	} else {
		responseCodeAsStr = strconv.Itoa(responseCode)
		responseName = "response " + responseCodeAsStr
	}
	return
}
