package shelf

import "os"

// writeFileAtomic writes a file through a temporary one, renamed once complete, so that a
// crash or a full disk never leaves a truncated file behind.
func writeFileAtomic(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		// Best effort: the write error is the one worth returning.
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}
