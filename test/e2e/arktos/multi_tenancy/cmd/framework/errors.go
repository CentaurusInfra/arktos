/*
Copyright 2020 Authors of Arktos.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"bytes"
	"fmt"
	"strings"
)

type ErrorList struct {
	errors []error
}

func NewErrorList(errors ...error) *ErrorList {
	return &ErrorList{errors: errors}
}

func (el *ErrorList) IsEmpty() bool {
	return len(el.errors) == 0
}

func (el *ErrorList) Add(err error) {
	if err != nil {
		el.errors = append(el.errors, err)
	}
}

func (el *ErrorList) Concat(errList *ErrorList) {
	if errList != nil {
		el.errors = append(el.errors, errList.errors...)
	}
}

func (el *ErrorList) String() string {
	var b bytes.Buffer
	b.WriteString("[")

	for i := 0; i < len(el.errors); i++ {
		b.WriteString(el.errors[i].Error())
		if i != len(el.errors)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("]")

	return b.String()
}

func ErrorIfNegative(name string, valuePtr *int) error {
	if valuePtr != nil && *valuePtr < 0 {
		return fmt.Errorf("Invalid %s: %v, cannot be negative", name, *valuePtr)
	}

	return nil
}

func ErrorIfEmpty(name string, str string) error {
	if strings.TrimSpace(str) == "" {
		return fmt.Errorf("Invalid %s, cannot be empty", name)
	}

	return nil
}

func ErrorIfOutOfBounds(name string, valuePtr *int, min int, max int) error {
	if valuePtr != nil && (*valuePtr < min || *valuePtr > max) {
		return fmt.Errorf("Invalid %s: %v, should be with [%d, %d]", name, *valuePtr, min, max)
	}

	return nil
}
