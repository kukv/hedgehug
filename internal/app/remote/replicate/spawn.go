package replicate

import "github.com/kukv/hedgehug/internal/lib/osfs"

// spawnDetached starts name with args in a session of its own, with no
// input and no output but its stderr, which goes to the log.
func spawnDetached(name string, args []string, logPath string) error {
	log, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }() // the child has its own copy
	return osfs.StartDetached(name, args, log)
}
