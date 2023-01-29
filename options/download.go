package options

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/mholt/archiver"
	"github.com/tychoish/fun/erc"
)

// Download represents the options to download a file to a given path and
// optionally extract its contents.
type Download struct {
	URL         string       `json:"url" bson:"url"`
	Path        string       `json:"path" bson:"path"`
	ArchiveOpts Archive      `json:"archive_opts" bson:"archive_opts"`
	HTTPClient  *http.Client `json:"-" bson:"-"`
}

// Validate checks the download options.
func (opts Download) Validate() error {
	catcher := &erc.Collector{}

	if opts.URL == "" {
		catcher.Add(errors.New("download url cannot be empty"))
	}

	if !filepath.IsAbs(opts.Path) {
		catcher.Add(errors.New("download path must be an absolute path"))
	}

	catcher.Add(opts.ArchiveOpts.Validate())

	return catcher.Resolve()
}

// Download executes the download operation.
func (opts Download) Download(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return fmt.Errorf("problem building request: %w", err)
	}

	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("problem downloading file for url %q: %w", opts.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: could not download %s to path %s", resp.Status, opts.URL, opts.Path)
	}

	if err = writeFile(resp.Body, opts.Path); err != nil {
		return err
	}

	if opts.ArchiveOpts.ShouldExtract {
		if err = opts.Extract(); err != nil {
			return fmt.Errorf("problem extracting file %q to path %q: %w", opts.Path, opts.ArchiveOpts.TargetPath, err)
		}
	}

	return nil
}

// Extract extracts the download to the path specified, using the archive format
// specified.
func (opts Download) Extract() error {
	var archiveHandler archiver.Unarchiver
	switch opts.ArchiveOpts.Format {
	case ArchiveAuto:
		unzipper, _ := archiver.ByExtension(opts.Path)
		if unzipper == nil {
			return fmt.Errorf("could not detect archive format for %s", opts.Path)
		}
		var ok bool
		archiveHandler, ok = unzipper.(archiver.Unarchiver)
		if !ok {
			return fmt.Errorf("%s was not a supported archive format [%T]", opts.Path, unzipper)
		}
	case ArchiveTarGz:
		archiveHandler = archiver.NewTarGz()
	case ArchiveZip:
		archiveHandler = archiver.NewZip()
	default:
		return fmt.Errorf("unrecognized archive format %s", opts.ArchiveOpts.Format)
	}

	if err := archiveHandler.Unarchive(opts.Path, opts.ArchiveOpts.TargetPath); err != nil {
		return fmt.Errorf("problem extracting archive %q to %q: %w", opts.Path, opts.ArchiveOpts.TargetPath, err)
	}

	return nil
}
