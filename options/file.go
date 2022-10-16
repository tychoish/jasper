package options

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"errors"

	"github.com/tychoish/emt"
)

// WriteFile represents the options necessary to write to a file.
type WriteFile struct {
	Path string `json:"path" bson:"path"`
	// File content can come from either Content or Reader, but not both.
	// Content should only be used if the entire file's contents can be held in
	// memory.
	Content []byte      `json:"content" bson:"content"`
	Reader  io.Reader   `json:"-" bson:"-"`
	Append  bool        `json:"append" bson:"append"`
	Perm    os.FileMode `json:"perm" bson:"perm"`
}

// validateContent ensures that there is at most one source of content for
// the file.
func (opts *WriteFile) validateContent() error {
	if len(opts.Content) > 0 && opts.Reader != nil {
		return errors.New("cannot have both data and reader set as file content")
	}
	// If neither is set, ensure that Content is empty rather than nil to
	// prevent potential writes with a nil slice.
	if len(opts.Content) == 0 && opts.Reader == nil {
		opts.Content = []byte{}
	}
	return nil
}

// Validate ensures that all the parameters to write to a file are valid and sets
// default permissions if necessary.
func (opts *WriteFile) Validate() error {
	if opts.Perm == 0 {
		opts.Perm = 0666
	}

	catcher := emt.NewBasicCatcher()
	catcher.NewWhen(opts.Path == "", "path to file must be specified")
	catcher.Add(opts.validateContent())
	return catcher.Resolve()
}

// DoWrite writes the data to the given path, creating the directory hierarchy as
// needed and the file if it does not exist yet.
func (opts *WriteFile) DoWrite() error {
	if err := makeEnclosingDirectories(filepath.Dir(opts.Path)); err != nil {
		return fmt.Errorf("problem making enclosing directories: %w", err)
	}

	openFlags := os.O_RDWR | os.O_CREATE
	if opts.Append {
		openFlags |= os.O_APPEND
	} else {
		openFlags |= os.O_TRUNC
	}

	file, err := os.OpenFile(opts.Path, openFlags, 0666)
	if err != nil {
		return fmt.Errorf("error opening file %q: %w", opts.Path, err)
	}

	catcher := emt.NewBasicCatcher()

	reader, err := opts.ContentReader()
	if err != nil {
		catcher.Errorf("error getting file content as bytes: %w", err)
		catcher.Add(file.Close())
		return catcher.Resolve()
	}

	bufReader := bufio.NewReader(reader)
	if _, err = io.Copy(file, bufReader); err != nil {
		catcher.Errorf("error writing content to file: %w", err)
		catcher.Add(file.Close())
		return catcher.Resolve()
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("closing %q: %w", opts.Path, err)
	}
	return nil
}

// WriteBufferedContent writes the content to a file by repeatedly calling
// doWrite with a buffered portion of the content. doWrite processes the
// WriteFile containing the next content to write to the file.
func (opts *WriteFile) WriteBufferedContent(doWrite func(bufopts WriteFile) error) error {
	if err := opts.validateContent(); err != nil {
		return fmt.Errorf("could not validate file content source: %w", err)
	}
	didWrite := false
	for buf, err := opts.contentBytes(); len(buf) != 0; buf, err = opts.contentBytes() {
		if err != nil && err != io.EOF {
			return fmt.Errorf("error getting content bytes: %w", err)
		}

		bufOpts := *opts
		bufOpts.Content = buf
		if didWrite {
			bufOpts.Append = true
		}

		if writeErr := doWrite(bufOpts); writeErr != nil {
			return fmt.Errorf("could not perform buffered write: %w", writeErr)
		}

		didWrite = true

		if err == io.EOF {
			break
		}
	}

	if didWrite {
		return nil
	}

	if err := doWrite(*opts); err != nil {
		return fmt.Errorf("could not perform buffered writes: %w", err)
	}
	return nil
}

// SetPerm sets the file permissions on the file. This should be called after
// DoWrite. If no file exists at (WriteFile).Path, it will error.
func (opts *WriteFile) SetPerm() error {
	if err := os.Chmod(opts.Path, opts.Perm); err != nil {
		return fmt.Errorf("error setting permissions: %w", err)
	}
	return nil
}

// contentBytes returns the contents to be written to the file as a byte slice.
// and will return io.EOF when all the file content has been received. Callers
// should process the byte slice before checking for the io.EOF condition.
func (opts *WriteFile) contentBytes() ([]byte, error) {
	if err := opts.validateContent(); err != nil {
		return nil, fmt.Errorf("could not validate file content source: %w", err)
	}

	if opts.Reader != nil {
		const mb = 1024 * 1024
		buf := make([]byte, mb)
		n, err := opts.Reader.Read(buf)
		return buf[:n], err
	}

	return opts.Content, io.EOF
}

// ContentReader returns the contents to be written to the file as an io.Reader.
func (opts *WriteFile) ContentReader() (io.Reader, error) {
	if err := opts.validateContent(); err != nil {
		return nil, fmt.Errorf("could not validate file content source: %w", err)
	}

	if opts.Reader != nil {
		return opts.Reader, nil
	}

	opts.Reader = bytes.NewBuffer(opts.Content)
	opts.Content = nil

	return opts.Reader, nil
}
