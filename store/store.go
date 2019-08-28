package store

import (
	"fmt"

	"github.com/keikoproj/active-monitor/api/v1alpha1"
)

// ArtifactReader enables reading artifacts from an external store
type ArtifactReader interface {
	Read() ([]byte, error)
}

// GetArtifactReader returns the ArtifactReader for this location
func GetArtifactReader(loc *v1alpha1.ArtifactLocation) (ArtifactReader, error) {
	if loc.Inline != nil {
		return NewInlineReader(loc.Inline)
	} else if loc.URL != nil {
		return NewURLReader(loc.URL)
	}
	return nil, fmt.Errorf("unknown artifact location: %v", *loc)
}
