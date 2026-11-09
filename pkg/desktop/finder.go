package desktop

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var openFinderCmd = defaultOpenFinder

// ShowInFileFinder reveals the specified file in the operating system's native file finder / file manager.
// - On macOS, it reveals the file in Finder (open -R).
// - On Windows, it highlights the file in File Explorer (explorer.exe /select,).
// - On Linux and other Unix-like systems, it calls FreeDesktop FileManager1 ShowItems via dbus-send,
//   falling back to xdg-open on the containing folder.
func ShowInFileFinder(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("empty file path")
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	target := absPath
	if _, err := os.Stat(target); os.IsNotExist(err) {
		dir := filepath.Dir(absPath)
		if _, err := os.Stat(dir); err == nil {
			target = dir
		} else {
			return fmt.Errorf("file does not exist: %s", absPath)
		}
	}

	return openFinderCmd(target)
}

func defaultOpenFinder(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", target).Start()
	case "windows":
		return exec.Command("explorer.exe", "/select,"+target).Start()
	default:
		// Convert file path to file:// URI for D-Bus
		u := &url.URL{
			Scheme: "file",
			Path:   filepath.ToSlash(target),
		}
		uri := u.String()

		// Attempt FreeDesktop FileManager1 ShowItems
		cmd := exec.Command("dbus-send", "--session", "--dest=org.freedesktop.FileManager1",
			"--type=method_call", "/org/freedesktop/FileManager1",
			"org.freedesktop.FileManager1.ShowItems",
			"array:string:"+uri, "string:")
		if err := cmd.Run(); err == nil {
			return nil
		}

		// Fallback: open parent folder via xdg-open
		dir := filepath.Dir(target)
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			dir = target
		}
		return exec.Command("xdg-open", dir).Start()
	}
}
