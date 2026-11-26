package coverage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fuvito/api-workbench/packages/core/serialization"
)

// Which spec an entry belongs to (B-080):
//
//   - a spec file that exists is known by the file itself, however it was written: its path inside
//     the workspace ("specs/api.yaml"), or its full path when it is outside;
//   - anything else, a web address first of all, is known by its exact text.
//
// That name is what the entry stores as SpecFile and shows, so two different specs are never one
// entry and one spec spelled two ways is never two.
//
// A relative path means nothing without the folder it is relative to, so callers that know it pass
// it as a base: the scenario's folder for a specFile, the current folder for a command-line path.

// Identify returns the name coverage is stored under for ref. bases are tried first, then the
// workspace folder, then the folders of the scenarios that write ref as their specFile.
func Identify(workspacePath, ref string, bases ...string) string {
	if ref == "" || isURL(ref) {
		return ref
	}
	if file := locate(workspacePath, ref, bases); file != "" {
		return nameOfFile(workspacePath, file)
	}
	// a full path still names one file when it is gone, so its entry can be found and reset
	if filepath.IsAbs(ref) {
		return nameOfFile(workspacePath, ref)
	}
	return ref
}

func isURL(ref string) bool { return strings.Contains(ref, "://") }

// locate finds the file ref means, or "" when there is none, or when the scenarios that write ref
// mean different files by it: then there is no way to know which one is meant.
func locate(workspacePath, ref string, bases []string) string {
	if filepath.IsAbs(ref) {
		if isFile(ref) {
			return ref
		}
		return ""
	}
	for _, base := range append(append([]string(nil), bases...), workspacePath) {
		if base == "" {
			continue
		}
		if file := filepath.Join(base, ref); isFile(file) {
			return file
		}
	}
	found := map[string]string{}
	for _, dir := range append(scenarioDirsWriting(workspacePath, ref), filepath.Join(workspacePath, "scenarios")) {
		if file := filepath.Join(dir, ref); isFile(file) {
			found[nameOfFile(workspacePath, file)] = file
		}
	}
	if len(found) != 1 {
		return ""
	}
	for _, file := range found {
		return file
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// real is the file's own spelling: absolute, links followed, and on Windows the case on disk.
func real(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	// a file that is gone: spell its folder as the disk does
	if dir := filepath.Dir(path); dir != path {
		return filepath.Join(real(dir), filepath.Base(path))
	}
	return filepath.Clean(path)
}

func nameOfFile(workspacePath, file string) string {
	file = real(file)
	if workspacePath != "" {
		rel, err := filepath.Rel(real(workspacePath), file)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

func scenarioDirsWriting(workspacePath, ref string) []string {
	if workspacePath == "" {
		return nil
	}
	seen := map[string]bool{}
	_ = filepath.WalkDir(filepath.Join(workspacePath, "scenarios"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml", ".json":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if sc, err := serialization.ParseScenarioFile(data, filepath.Base(path)); err == nil && sc.SpecFile == ref {
			seen[filepath.Dir(path)] = true
		}
		return nil
	})
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// Key is the file name of the entry for a spec name returned by Identify: readable, and told apart
// from every other name by a hash of all of it.
func Key(name string) string {
	sum := sha256.Sum256([]byte(name))
	readable := strings.TrimSuffix(legacyKey(name), ".json")
	if len(readable) > 60 {
		readable = readable[len(readable)-60:]
	}
	return strings.TrimLeft(readable, "_") + "-" + hex.EncodeToString(sum[:6]) + ".json"
}

// legacyKey is the file name releases up to 0.13.5 used: made from the text as written, with
// punctuation and case lost and only the last 80 characters kept, so different specs could share
// one entry.
func legacyKey(specFile string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(specFile) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	s = strings.Trim(s, "_")
	if len(s) > 80 {
		s = s[len(s)-80:]
	}
	return s + ".json"
}

func read(path string) (StoredCoverage, bool) {
	var cov StoredCoverage
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &cov) != nil {
		return StoredCoverage{}, false
	}
	return cov, true
}

// isLegacy reports an entry still under the file name an older release gave it.
func isLegacy(fileName string, cov StoredCoverage) bool {
	return cov.SpecFile != "" && fileName == legacyKey(cov.SpecFile)
}

// rename moves an old entry to its new name. It never replaces an entry: when one is already there
// (an older release ran again after the move), the old one is left as it is.
func rename(workspacePath, oldPath string, cov StoredCoverage, name string) bool {
	target := filepath.Join(DirPath(workspacePath), Key(name))
	if _, err := os.Stat(target); err == nil {
		return false
	}
	cov.SpecFile = name
	if write(workspacePath, target, cov) != nil {
		return false
	}
	return os.Remove(oldPath) == nil
}

// upgrade renames every old entry whose spec can be told for certain: a web address, or a path that
// means exactly one file. The others stay, readable under their old name.
func upgrade(workspacePath string) {
	entries, err := os.ReadDir(DirPath(workspacePath))
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(DirPath(workspacePath), e.Name())
		cov, ok := read(path)
		if !ok || !isLegacy(e.Name(), cov) {
			continue
		}
		if !isURL(cov.SpecFile) && locate(workspacePath, cov.SpecFile, nil) == "" {
			continue
		}
		rename(workspacePath, path, cov, Identify(workspacePath, cov.SpecFile))
	}
}

// find returns the path and content of the entry for ref, upgrading old entries on the way.
func find(workspacePath, ref string, bases []string) (string, StoredCoverage, bool) {
	upgrade(workspacePath)
	name := Identify(workspacePath, ref, bases...)
	path := filepath.Join(DirPath(workspacePath), Key(name))
	if cov, ok := read(path); ok {
		cov.SpecFile = name
		return path, cov, true
	}
	// An old entry the workspace alone could not place, which this caller's folder can.
	for _, written := range []string{ref, name} {
		old := filepath.Join(DirPath(workspacePath), legacyKey(written))
		cov, ok := read(old)
		if !ok || cov.SpecFile != written || !isLegacy(legacyKey(written), cov) {
			continue
		}
		if locate(workspacePath, written, nil) == "" && rename(workspacePath, old, cov, name) {
			cov.SpecFile = name
			return path, cov, true
		}
		return old, cov, true
	}
	return path, StoredCoverage{SpecFile: name}, false
}
