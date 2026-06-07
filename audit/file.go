package audit

import (
	"encoding/json"
	"io"
	"os"
)

// OpenFile opens (or creates) path for appending and returns a Handler that
// continues the chain already in the file. If the file is non-empty its last
// entry is read to recover the next seq and the prev hash, so restarts keep one
// unbroken chain instead of starting a fresh genesis each time.
//
// The returned *os.File is the caller's to close. opts.StartSeq and
// opts.PrevHash are ignored: they are recovered from the file.
func OpenFile(path string, opts *Options) (*Handler, *os.File, error) {
	// O_RDWR (not O_WRONLY) so lastLine's ReadAt can read the tail to resume the
	// chain. O_APPEND still forces every write to the end regardless.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	resume := Options{}
	if opts != nil {
		resume.Signer = opts.Signer
		resume.Level = opts.Level
	}

	if fi.Size() > 0 {
		line, err := lastLine(f, fi.Size())
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		if len(line) > 0 {
			var last Entry
			if err := json.Unmarshal(line, &last); err != nil {
				_ = f.Close()
				return nil, nil, err
			}
			resume.StartSeq = last.Seq + 1
			resume.PrevHash = last.Hash
		}
	}

	h, err := NewHandler(f, &resume)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return h, f, nil
}

// lastLine returns the last non-empty line of f without reading the whole file,
// scanning backward from the end in fixed chunks until it finds the start of
// the final line.
func lastLine(f *os.File, size int64) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}
	const chunk = 4096
	end := size // exclusive end of the last line's content (trailing newlines trimmed)
	var lineStart int64 = -1
	stripping := true
	pos := size

	for pos > 0 {
		readSize := int64(chunk)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, pos); err != nil && err != io.EOF {
			return nil, err
		}
		for i := len(buf) - 1; i >= 0; i-- {
			c := buf[i]
			abs := pos + int64(i)
			if stripping {
				if c == '\n' || c == '\r' {
					end = abs
					continue
				}
				stripping = false
			}
			if c == '\n' {
				lineStart = abs + 1
				goto done
			}
		}
	}

done:
	if lineStart < 0 {
		lineStart = 0
	}
	length := end - lineStart
	if length <= 0 {
		return nil, nil
	}
	out := make([]byte, length)
	if _, err := f.ReadAt(out, lineStart); err != nil && err != io.EOF {
		return nil, err
	}
	return out, nil
}
