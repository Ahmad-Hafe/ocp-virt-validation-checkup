package junit

import (
	"os"
	"path"
	"strings"
	"testing"
)

func TestReadTestSuiteAggregatesMultipleSuites(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="3" failures="1" errors="0" skipped="0">
  <testsuite name="pytest" tests="1" failures="0" errors="0" skipped="0" time="0.100">
    <testcase classname="tests.storage.test_a" name="test_path" time="0.100"/>
  </testsuite>
  <testsuite name="pytest-windows" tests="2" failures="1" errors="0" skipped="0" time="0.200">
    <testcase classname="tests.virt.test_b" name="test_win_ok" time="0.100"/>
    <testcase classname="tests.virt.test_c" name="test_win_fail" time="0.100">
      <failure>windows failed</failure>
    </testcase>
  </testsuite>
</testsuites>`

	suite, err := readTestSuite(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if suite.Tests != 3 {
		t.Errorf("expected 3 tests, got %d", suite.Tests)
	}
	if suite.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", suite.Failures)
	}
	if len(suite.TestCases) != 3 {
		t.Errorf("expected 3 test cases, got %d", len(suite.TestCases))
	}

	names := map[string]bool{}
	for _, tc := range suite.TestCases {
		names[tc.Name] = true
	}
	for _, want := range []string{"test_path", "test_win_ok", "test_win_fail"} {
		if !names[want] {
			t.Errorf("expected test case %q in aggregated suite", want)
		}
	}
	if !suite.TestCases[2].Failure {
		t.Errorf("expected Windows failure case to retain <failure>")
	}
}

func generateResDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for subDir, content := range files {
		subDirPath := path.Join(dir, subDir)
		err := os.Mkdir(subDirPath, 0755)
		if err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		if len(content) == 0 {
			continue
		}

		filePath := path.Join(subDirPath, junitFileName)
		err = os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}
	return dir
}

func TestNewResultMapWithValidFiles(t *testing.T) {
	dir := generateResDir(t, map[string]string{
		"sig1": `<testsuite name="suite1"></testsuite>`,
		"sig2": `<testsuite name="suite2"></testsuite>`,
	})

	result, err := NewResultMap(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	if _, ok := result["sig1"]; !ok {
		t.Errorf("expected result for sig1")
	}

	if _, ok := result["sig2"]; !ok {
		t.Errorf("expected result for sig2")
	}
}

func TestNewResultMapWithEmptyDirectory(t *testing.T) {
	dir := generateResDir(t, nil)

	result, err := NewResultMap(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestNewResultMapWithMissingJunitFiles(t *testing.T) {
	dir := generateResDir(t, map[string]string{"sig1": "", "sig2": ""})

	result, err := NewResultMap(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestNewResultMapWithInvalidJunitFiles(t *testing.T) {
	dir := generateResDir(t, map[string]string{
		"sig1": `<invalid>`,
		"sig2": `<invalid>`,
	})

	result, err := NewResultMap(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestNewResultMapSkipsLostAndFoundDirectory(t *testing.T) {
	validJunitContent := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite tests="1" failures="0" time="0.001" name="test.suite">
    <testcase classname="test.class" name="test.name" time="0.001"></testcase>
</testsuite>`

	dir := generateResDir(t, map[string]string{
		"sig1": validJunitContent,
		"sig2": validJunitContent,
	})

	// Create a lost+found directory (without junit file)
	lostFoundPath := path.Join(dir, "lost+found")
	err := os.Mkdir(lostFoundPath, 0755)
	if err != nil {
		t.Fatalf("failed to create lost+found directory: %v", err)
	}

	result, err := NewResultMap(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have 2 results (sig1 and sig2), lost+found should be skipped
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	// Verify that lost+found is not in the results
	if _, exists := result["lost+found"]; exists {
		t.Error("lost+found directory should have been skipped but was found in results")
	}

	// Verify that the valid directories are present
	if _, exists := result["sig1"]; !exists {
		t.Error("sig1 should be present in results but was not found")
	}
	if _, exists := result["sig2"]; !exists {
		t.Error("sig2 should be present in results but was not found")
	}
}
