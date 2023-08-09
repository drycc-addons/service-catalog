/*
Copyright 2018 The Kubernetes Authors.

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

package test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// UpdateGolden writes out the golden files with the latest values, rather than failing the test.
// Example: go test ./cmd/svcat --update
var UpdateGolden = flag.Bool("update", false, "update golden files")

// buildTestdataPath returns the full path to a testdata file.
// * relpath - relative path to the file in the test's testdata directory.
func buildTestdataPath(relpath string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to get the current working directory: %w", err)
	}

	path := filepath.Join(pwd, "testdata", relpath)
	return path, nil
}

// GetTestdata returns the contents of a testdata file.
// * relpath - relative path to the file in the test's testdata directory.
func GetTestdata(relpath string) (fullpath string, contents []byte, err error) {
	fullpath, err = buildTestdataPath(relpath)
	if err != nil {
		return "", nil, err
	}

	contents, err = os.ReadFile(fullpath)
	if err != nil {
		err = fmt.Errorf("unable to read testdata %s: %w", fullpath, err)
	}
	return fullpath, contents, err
}

// AssertEqualsGoldenFile asserts that the value equals the contents of the golden file.
// When the go test -update flag is present, the golden file is updated to match, rather than failing the test.
func AssertEqualsGoldenFile(t *testing.T, goldenFile string, got string) {
	t.Helper()

	path, want, err := GetTestdata(goldenFile)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	gotB := []byte(got)
	if !bytes.Equal(want, gotB) {
		if *UpdateGolden {
			err := os.WriteFile(path, gotB, 0666)
			if err != nil {
				t.Fatalf("%+v", fmt.Errorf("unable to update golden file %s: %w", path, err))
			}
		} else {
			t.Fatalf("does not match golden file %s\n\nWANT:\n%q\n\nGOT:\n%q\n\nSee https://service-catalog.drycc.cc/docs/devguide/#golden-files for how to work with golden files.", path, want, gotB)
		}
	}
}
