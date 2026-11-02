package releaseenv

import (
	"strings"
	"testing"
)

func TestCoverage90BodyIncludesFlatArtifactSource(t *testing.T) {
	body := Body(Record{ArtifactSource: " https://mirror.test/release/ "})
	if !strings.Contains(body, "PYXIS_ARTIFACT_SOURCE=https://mirror.test/release\n") {
		t.Fatalf("Body = %q", body)
	}
}
