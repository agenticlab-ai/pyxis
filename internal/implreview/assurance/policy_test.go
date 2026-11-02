package assurance

import "testing"

func TestRequiredExhaustiveAssuranceBandMatrix(t *testing.T) {
	tests := []struct {
		assurance string
		band      string
		depth     string
		passes    int
	}{
		{"ordinary", "skip", "standard", 1},
		{"ordinary", "standard", "standard", 1},
		{"ordinary", "deep", "deep", 1},
		{"elevated", "skip", "deep", 1},
		{"elevated", "standard", "deep", 1},
		{"elevated", "deep", "deep", 1},
		{"critical", "skip", "deep", 2},
		{"critical", "standard", "deep", 2},
		{"critical", "deep", "deep", 2},
	}
	for _, tc := range tests {
		t.Run(tc.assurance+"/"+tc.band, func(t *testing.T) {
			depth, passes, err := Required(tc.assurance, tc.band)
			if err != nil || depth != tc.depth || passes != tc.passes {
				t.Fatalf("Required = %q x %d, %v; want %q x %d", depth, passes, err, tc.depth, tc.passes)
			}
		})
	}
	if _, _, err := Required("unknown", "deep"); err == nil {
		t.Fatal("unknown assurance was accepted")
	}
}
