package fakecalibreweb

import (
	"archive/zip"
	"bytes"
)

// storedFile is a file stored uncompressed at the start of a zip.
type storedFile struct {
	name string
	data []byte
}

// zipFiles returns a zip of first, stored in their order, then of files, compressed.
func zipFiles(first []storedFile, files map[string][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range first {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: file.name, Method: zip.Store})
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(file.data); err != nil {
			return nil, err
		}
	}
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
