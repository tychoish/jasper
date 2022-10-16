package options

import (
	"fmt"
	"path/filepath"

	"errors"

	"github.com/tychoish/emt"
)

// ArchiveFormat represents an archive file type.
type ArchiveFormat string

const (
	// ArchiveAuto is an ArchiveFormat that does not force any particular type of
	// archive format.
	ArchiveAuto ArchiveFormat = "auto"
	// ArchiveTarGz is an ArchiveFormat for gzipped tar archives.
	ArchiveTarGz ArchiveFormat = "targz"
	// ArchiveZip is an ArchiveFormat for Zip archives.
	ArchiveZip ArchiveFormat = "zip"
)

// Validate checks that the ArchiveFormat is a recognized format.
func (f ArchiveFormat) Validate() error {
	switch f {
	case ArchiveTarGz, ArchiveZip, ArchiveAuto:
		return nil
	default:
		return fmt.Errorf("unknown archive format %s", f)
	}
}

// Archive encapsulates options related to management of archive files.
type Archive struct {
	ShouldExtract bool          `bson:"should_extract" json:"should_extract" yaml:"should_extract"`
	Format        ArchiveFormat `bson:"format" json:"format" yaml:"format"`
	TargetPath    string        `bson:"target_path" json:"target_path" yaml:"target_path"`
}

// Validate checks the archive file options.
func (opts Archive) Validate() error {
	if !opts.ShouldExtract {
		return nil
	}

	catcher := emt.NewBasicCatcher()

	if !filepath.IsAbs(opts.TargetPath) {
		catcher.Add(errors.New("download path must be an absolute path"))
	}

	catcher.Add(opts.Format.Validate())

	return catcher.Resolve()
}
