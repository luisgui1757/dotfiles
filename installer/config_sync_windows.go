package installer

import "os"

// Windows directory handles cannot be flushed with FlushFileBuffers. Payload
// files are flushed before handle-relative publication, as in the state writer.
func syncConfigDirectory(string) error { return nil }

func syncConfigHandle(*os.File) error { return nil }
