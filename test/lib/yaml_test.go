package lib_test

import "testing"
import "reflect"
import "github.com/OSU-SOC/nagini/lib"

// Test the ParseConfig command.
// Runs the command, and passes in various test.yaml
// files to parse. Compares them to an expected struct.
func TestParseConfig(t *testing.T) {
	type testEntry struct {
		input			string
		expectedData	lib.Config
		expectedErr		error
	}

	// Test Table to loop over.
	testTable := []testEntry{
		// TEST #1
		{
			input: "test1.yaml",
			expectedData: lib.Config {
				DataSources: []lib.DataSource{
					{
						Name: "test_name",
						Threads: 6,
						ManualPath: "/a/b",
					},
				},
			},
			expectedErr: nil,
		},
		// TEST #2 ...
	}

	// Run function over test table
	for _, testCase := range testTable {
		t.Run(testCase.input, func(t *testing.T) {
			actualData, actualErr := lib.ParseConfig(testCase.input)
			if !reflect.DeepEqual(actualData, testCase.expectedData) {
				t.Errorf("\nIncorrect Data.\nexpected %v\ngot %v", testCase.expectedData, actualErr)
			} else if actualErr != testCase.expectedErr {
				t.Errorf("\nIncorrect Error.\nexpected %v\ngot %v", testCase.expectedData, actualErr)
			}
		})
	}
}