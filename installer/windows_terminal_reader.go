package installer

// A reader exists for one extraction of unchanged input only. The key names the
// exact slice window without copying or hashing its bytes; no unsafe arithmetic
// is needed. Writers use the uncached helpers, so edits cannot reuse stale parses.
type windowsTerminalReadKey struct {
	start  *byte
	length int
}

func terminalReadKey(data []byte) windowsTerminalReadKey {
	key := windowsTerminalReadKey{length: len(data)}
	if len(data) != 0 {
		key.start = &data[0]
	}
	return key
}

type windowsTerminalReadArray struct {
	elements []windowsTerminalElement
	lexical  []byte
}

type windowsTerminalReader struct {
	properties map[windowsTerminalReadKey][]jsonProperty
	arrays     map[windowsTerminalReadKey]windowsTerminalReadArray
}

func (r *windowsTerminalReader) propertiesOf(data []byte) ([]jsonProperty, error) {
	key := terminalReadKey(data)
	if properties, ok := r.properties[key]; ok {
		return properties, nil
	}
	properties, err := windowsTerminalProperties(data)
	if err != nil {
		return nil, err
	}
	if r.properties == nil {
		r.properties = make(map[windowsTerminalReadKey][]jsonProperty)
	}
	r.properties[key] = properties
	return properties, nil
}

func (r *windowsTerminalReader) elementsOf(data []byte) ([]windowsTerminalElement, []byte, error) {
	key := terminalReadKey(data)
	if array, ok := r.arrays[key]; ok {
		return array.elements, array.lexical, nil
	}
	elements, lexical, err := windowsTerminalElements(data)
	if err != nil {
		return nil, nil, err
	}
	if r.arrays == nil {
		r.arrays = make(map[windowsTerminalReadKey]windowsTerminalReadArray)
	}
	r.arrays[key] = windowsTerminalReadArray{elements, lexical}
	return elements, lexical, nil
}

func (r *windowsTerminalReader) get(data []byte, path string) ([]byte, bool, error) {
	return windowsTerminalLookup(data, path, r.propertiesOf)
}

func (r *windowsTerminalReader) entry(array []byte, collection windowsTerminalCollection) ([]byte, int, error) {
	return windowsTerminalFindEntry(array, collection, r.elementsOf, r.get)
}
