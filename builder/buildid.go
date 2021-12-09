package builder

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
)

// ReadBuildID reads the build ID from the currently running executable.
func ReadBuildID() ([]byte, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(executable)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	switch runtime.GOOS {
	case "linux", "freebsd":
		// Read the GNU build id section. (Not sure about FreeBSD though...)
		file, err := elf.NewFile(f)
		if err != nil {
			return nil, err
		}
		for _, section := range file.Sections {
			if section.Type != elf.SHT_NOTE || section.Name != ".note.gnu.build-id" {
				continue
			}
			buf := make([]byte, section.Size)
			n, err := section.ReadAt(buf, 0)
			if uint64(n) != section.Size || err != nil {
				return nil, fmt.Errorf("could not read build id: %w", err)
			}
			return buf, nil
		}
	case "darwin":
		// Read the LC_UUID load command, which contains the equivalent of a
		// build ID.
		file, err := macho.NewFile(f)
		if err != nil {
			return nil, err
		}
		for _, load := range file.Loads {
			// Unfortunately, the debug/macho package doesn't support the
			// LC_UUID command directly. So we have to read it from
			// macho.LoadBytes.
			load, ok := load.(macho.LoadBytes)
			if !ok {
				continue
			}
			raw := load.Raw()
			command := binary.LittleEndian.Uint32(raw)
			if command != 0x1b {
				// Looking for the LC_UUID load command.
				// LC_UUID is defined here as 0x1b:
				// https://opensource.apple.com/source/xnu/xnu-4570.71.2/EXTERNAL_HEADERS/mach-o/loader.h.auto.html
				continue
			}
			return raw[4:], nil
		}
	case "windows":
		// Unfortunately, Windows doesn't seem to have an equivalent of a build
		// ID. Luckily, Go does have an equivalent of the build ID, which is
		// stored as a special symbol named go.buildid. You can read it using
		// `go tool buildid`, but the code below extracts it directly from the
		// binary.
		// The code below is a lot more sophisticated though: `go tool buildid`
		// simply scans the binary for the "\xff Go build ID: " string while we
		// use the symbol table for this purpose.
		file, err := pe.NewFile(f)
		if err != nil {
			return nil, err
		}
		for _, symbol := range file.Symbols {
			if symbol.Name != "go.buildid" {
				// Not the build ID.
				continue
			}
			// The symbol happens to be exactly 100 bytes long.
			buf := make([]byte, 100)
			section := file.Sections[symbol.SectionNumber-1]
			n, err := section.ReadAt(buf, int64(symbol.Value))
			if err != nil || n != len(buf) {
				return nil, fmt.Errorf("could not read build id: %w", err)
			}
			if bytes.HasPrefix(buf, []byte("\xff Go build ID: \"")) && buf[len(buf)-1] == '"' {
				return buf[len("\xff Go build ID: \"") : len(buf)-1], nil
			}
		}
	default:
		return nil, fmt.Errorf("cannot read build ID for GOOS=%s", runtime.GOOS)
	}
	return nil, fmt.Errorf("could not find build ID in %s", executable)
}
