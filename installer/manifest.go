package installer

import _ "embed"

//go:embed resources.json
var resourceData []byte

func DefaultCatalog() (*Catalog, error) {
	var catalog Catalog
	if err := Decode(resourceData, &catalog); err != nil {
		return nil, err
	}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return &catalog, nil
}
