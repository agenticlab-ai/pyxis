package matrix

import "testing"

// stackFixtures returns the three fixture stacks and every scenario the matrix
// (legitimate) and catch suite (adversarial) drive each through. This is DATA:
// the stack-specific content lives here in the fixtures, never in the harness
// (internal/verify), which the inspection test asserts. Every stack carries the
// same scenario shape — clean, three legitimate diffs, and the three seed
// families — so the driver iterates them uniformly with no per-stack branch.
func stackFixtures() []stackFixture {
	return []stackFixture{goFixture(), pythonFixture(), nodeFixture()}
}

// machineryScenario is the rung-0 config-pin downgrade — identical on every
// stack (machinery protection is path-based, DINV-2), so it is shared.
func machineryScenario() scenario {
	return scenario{
		name: "machinery-pin-downgrade",
		baseSetup: func(t *testing.T, dir string) {
			writeFixtureFile(t, dir, ".pyxis/config.yaml", "codegate:\n  version: \"1.0.0\"\n")
		},
		seed: func(t *testing.T, dir string) {
			writeFixtureFile(t, dir, ".pyxis/config.yaml", "codegate:\n  version: \"0.0.1\"\n")
		},
		wantState: "red", wantClass: "machinery", wantRung: "0",
		family: "machinery", seedCase: "pin-downgrade",
	}
}

// --- go (fixture-go: package main, greeting(); no committed test file, so the
// silenced-test seeds add one via baseSetup) --------------------------------

func goFixture() stackFixture {
	return stackFixture{
		stack: "go", dir: "../testdata/fixture-go", tool: "go",
		scenarios: []scenario{
			{name: "clean", wantState: "green-degraded"},
			{name: "refactor-only", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(greeting())\n}\n\n// greeting returns fixture-go's fixed greeting.\nfunc greeting() string {\n\treturn \"fixture-go\" + \": hello\"\n}\n")
				}},
			{name: "test-only", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "greeting_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {\n\tif greeting() == \"\" {\n\t\tt.Fatal(\"empty greeting\")\n\t}\n}\n")
				}},
			{name: "mixed-genuine", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					appendFixtureFile(t, dir, "main.go", "\nfunc Double(n int) int {\n\treturn n * 2\n}\n")
					writeFixtureFile(t, dir, "double_test.go", "package main\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(3) != 6 {\n\t\tt.Fatal(\"Double wrong\")\n\t}\n}\n")
				}},
			{name: "tautology", wantState: "red", wantClass: "tautological-test", wantRung: "3", family: "tautology", seedCase: "tautology",
				seed: func(t *testing.T, dir string) {
					appendFixtureFile(t, dir, "main.go", "\nfunc Unused() int {\n\treturn 42\n}\n")
					writeFixtureFile(t, dir, "taut_test.go", "package main\n\nimport \"testing\"\n\nfunc TestTaut(t *testing.T) {\n\tif 1 != 1 {\n\t\tt.Fatal(\"impossible\")\n\t}\n}\n")
				}},
			{name: "silenced-deletion", wantState: "red", wantClass: "test-deleted", wantRung: "3", family: "silenced-test", seedCase: "deletion",
				baseSetup: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "greeting_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {\n\tif greeting() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n")
				},
				seed: func(t *testing.T, dir string) { removeFixtureFile(t, dir, "greeting_test.go") }},
			{name: "silenced-skip-marker", wantState: "red", wantClass: "skip-marker-added", wantRung: "3", family: "silenced-test", seedCase: "skip-marker",
				baseSetup: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "greeting_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {\n\tif greeting() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n")
				},
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "greeting_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {\n\tt.Skip(\"disabled\")\n\tif greeting() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n")
				}},
			{name: "silenced-assertion-removal", wantState: "red", wantClass: "assertion-count-drop", wantRung: "3", family: "silenced-test", seedCase: "assertion-removal",
				baseSetup: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "greeting_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {\n\tif greeting() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n\tif greeting() == \"x\" {\n\t\tt.Error(\"weird\")\n\t}\n}\n")
				},
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "greeting_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreeting(t *testing.T) {\n\t_ = greeting()\n}\n")
				}},
			machineryScenario(),
		},
	}
}

// --- python (fixture-python: calc.py + test_calc.py; the silenced-test seeds
// edit the fixture's own committed test) ------------------------------------

func pythonFixture() stackFixture {
	return stackFixture{
		stack: "python", dir: "../testdata/fixture-python", tool: "python3",
		scenarios: []scenario{
			{name: "clean", wantState: "green-degraded"},
			{name: "refactor-only", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "calc.py", "def add(a, b):\n    return b + a\n")
				}},
			{name: "test-only", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "test_extra.py", "from calc import add\n\nassert add(2, 2) == 4\n")
				}},
			{name: "mixed-genuine", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					appendFixtureFile(t, dir, "calc.py", "\n\ndef multiply(a, b):\n    return a * b\n")
					writeFixtureFile(t, dir, "test_multiply.py", "from calc import multiply\n\nassert multiply(2, 3) == 6\n")
				}},
			{name: "tautology", wantState: "red", wantClass: "tautological-test", wantRung: "3", family: "tautology", seedCase: "tautology",
				seed: func(t *testing.T, dir string) {
					appendFixtureFile(t, dir, "calc.py", "\n\ndef unused():\n    return 42\n")
					writeFixtureFile(t, dir, "test_taut.py", "assert 1 == 1\n")
				}},
			{name: "silenced-deletion", wantState: "red", wantClass: "test-deleted", wantRung: "3", family: "silenced-test", seedCase: "deletion",
				seed: func(t *testing.T, dir string) { removeFixtureFile(t, dir, "test_calc.py") }},
			{name: "silenced-skip-marker", wantState: "red", wantClass: "skip-marker-added", wantRung: "3", family: "silenced-test", seedCase: "skip-marker",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "test_calc.py", "import unittest\nfrom calc import add\n\n\n@unittest.skip(\"flaky\")\ndef test_add():\n    assert add(1, 2) == 3\n")
				}},
			{name: "silenced-assertion-removal", wantState: "red", wantClass: "assertion-count-drop", wantRung: "3", family: "silenced-test", seedCase: "assertion-removal",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "test_calc.py", "from calc import add\n\nresult = add(1, 2)\n")
				}},
			machineryScenario(),
		},
	}
}

// --- node (fixture-node: calc.js + calc.test.js + node_modules/leftpad state
// dir; the silenced-test seeds edit the fixture's own committed test) --------

func nodeFixture() stackFixture {
	return stackFixture{
		stack: "node", dir: "../testdata/fixture-node", tool: "node",
		scenarios: []scenario{
			{name: "clean", wantState: "green-degraded"},
			{name: "refactor-only", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "calc.js", "const leftpad = require('leftpad');\n\nfunction add(a, b) {\n  return b + a;\n}\n\nfunction padded(s) {\n  return leftpad(s, 5);\n}\n\nmodule.exports = { add, padded };\n")
				}},
			{name: "test-only", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "extra.test.js", "const assert = require('assert');\nconst { add } = require('./calc');\n\nassert.strictEqual(add(2, 2), 4);\n")
				}},
			{name: "mixed-genuine", wantState: "green-degraded",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "calc.js", "const leftpad = require('leftpad');\n\nfunction add(a, b) {\n  return a + b;\n}\n\nfunction padded(s) {\n  return leftpad(s, 5);\n}\n\nfunction multiply(a, b) {\n  return a * b;\n}\n\nmodule.exports = { add, padded, multiply };\n")
					writeFixtureFile(t, dir, "multiply.test.js", "const assert = require('assert');\nconst { multiply } = require('./calc');\n\nassert.strictEqual(multiply(2, 3), 6);\n")
				}},
			{name: "tautology", wantState: "red", wantClass: "tautological-test", wantRung: "3", family: "tautology", seedCase: "tautology",
				seed: func(t *testing.T, dir string) {
					appendFixtureFile(t, dir, "calc.js", "\nfunction unused() {\n  return 42;\n}\n")
					writeFixtureFile(t, dir, "taut.test.js", "const assert = require('assert');\n\nassert.strictEqual(1, 1);\n")
				}},
			{name: "silenced-deletion", wantState: "red", wantClass: "test-deleted", wantRung: "3", family: "silenced-test", seedCase: "deletion",
				seed: func(t *testing.T, dir string) { removeFixtureFile(t, dir, "calc.test.js") }},
			{name: "silenced-skip-marker", wantState: "red", wantClass: "skip-marker-added", wantRung: "3", family: "silenced-test", seedCase: "skip-marker",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "calc.test.js", "const assert = require('assert');\nconst { add, padded } = require('./calc');\n\nassert.strictEqual(padded('hi'), '   hi');\nit.skip('adds', () => {\n  assert.strictEqual(add(1, 2), 3);\n});\n")
				}},
			{name: "silenced-assertion-removal", wantState: "red", wantClass: "assertion-count-drop", wantRung: "3", family: "silenced-test", seedCase: "assertion-removal",
				seed: func(t *testing.T, dir string) {
					writeFixtureFile(t, dir, "calc.test.js", "const assert = require('assert');\nconst { padded } = require('./calc');\n\nassert.strictEqual(padded('hi'), '   hi');\n")
				}},
			machineryScenario(),
		},
	}
}
