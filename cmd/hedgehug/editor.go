package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// defaultEditor is the editor when neither $VISUAL nor $EDITOR is set.
var defaultEditor = func() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}()

// editor returns the editor command and its arguments: $VISUAL, then
// $EDITOR, then defaultEditor.
func (a *app) editor() ([]string, error) {
	for _, k := range []string{"VISUAL", "EDITOR"} {
		v := a.getenv(k)
		if v == "" {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			return nil, fmt.Errorf("$%s is blank; set it to an editor", k)
		}
		return f, nil
	}
	return []string{defaultEditor}, nil
}

// editFile writes content to a new temporary file hedgehug-<name>-*.md,
// opens it in the editor and returns what it holds afterwards. The file is
// left for the caller to remove when it is done with it; when the editor
// fails, the error names the file.
func (a *app) editFile(name string, content []byte) (path string, edited []byte, err error) {
	f, err := os.CreateTemp("", "hedgehug-"+name+"-*.md")
	if err != nil {
		return "", nil, err
	}
	path = f.Name()
	_, err = f.Write(content)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return path, nil, err
	}
	edited, err = a.openEditor(path)
	return path, edited, err
}

// reopen writes content to path again and opens it in the editor.
func (a *app) reopen(path string, content []byte) ([]byte, error) {
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return nil, err
	}
	return a.openEditor(path)
}

// openEditor runs the editor on path and reads the file back.
func (a *app) openEditor(path string) ([]byte, error) {
	ed, err := a.editor()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(ed[0], append(ed[1:], path)...) //nolint:gosec // the editor the user chose in $VISUAL or $EDITOR
	cmd.Env = os.Environ()
	// The editor gets stdin only when it is a file such as the terminal:
	// copying from any other reader would take the answers meant for
	// askRetry. stdout is stderr, so as not to mix into the JSON output.
	if f, ok := a.stdin.(*os.File); ok {
		cmd.Stdin = f
	}
	cmd.Stdout, cmd.Stderr = a.stderr, a.stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("the editor %s failed: %w; the file is kept at %s", ed[0], err, path)
		}
		return nil, fmt.Errorf("cannot run the editor %s: %w; set $EDITOR to an editor (the file is kept at %s)", ed[0], err, path)
	}
	return os.ReadFile(path) //nolint:gosec // the temporary file made by editFile
}

// askRetry asks on the terminal whether to open the editor again. An empty
// answer is yes. It is false when stdin is not a terminal.
func (a *app) askRetry() bool {
	if !a.isTerminal(a.stdin) {
		return false
	}
	if _, err := fmt.Fprint(a.stderr, "Open the editor again? [Y/n] "); err != nil {
		return false
	}
	line, ok := readLine(a.stdin)
	if !ok {
		return false // the end of the input, not an empty answer
	}
	ans := strings.TrimSpace(line)
	return ans == "" || ans == "y" || ans == "Y"
}

// readLine reads up to a newline one byte at a time, so nothing after the
// line is taken from r. ok is false when r ends before anything is read.
func readLine(r io.Reader) (line string, ok bool) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n == 1 {
			if buf[0] == '\n' {
				return b.String(), true
			}
			b.WriteByte(buf[0])
		}
		if err != nil {
			return b.String(), b.Len() > 0
		}
	}
}
