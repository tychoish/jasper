package options

import (
	"net/http"
	"path/filepath"

	"github.com/deciduosity/grip"
	"github.com/deciduosity/utility"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
)

// Download represents the options to download a file to a given path and
// optionally extract its contents.
type Download struct {
	URL         string  `json:"url" bson:"url"`
	Path        string  `json:"path" bson:"path"`
	ArchiveOpts Archive `json:"archive_opts" bson:"archive_opts"`
}

// Validate checks the download options.
func (opts Download) Validate() error {
	catcher := grip.NewBasicCatcher()

	if opts.URL == "" {
		catcher.New("download url cannot be empty")
	}

	if !filepath.IsAbs(opts.Path) {
		catcher.New("download path must be an absolute path")
	}

	catcher.Add(opts.ArchiveOpts.Validate())

	return catcher.Resolve()
}

// Download executes the download operation.
func (opts Download) Download() error {
	req, err := http.NewRequest(http.MethodGet, opts.URL, nil)
	if err != nil {
		return errors.Wrap(err, "problem building request")
	}

	client := utility.GetHTTPClient()
	defer utility.PutHTTPClient(client)

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "problem downloading file for url %s", opts.URL)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("%s: could not download %s to path %s", resp.Status, opts.URL, opts.Path)
	}

	if err = writeFile(resp.Body, opts.Path); err != nil {
		return err
	}

	if opts.ArchiveOpts.ShouldExtract {
		if err = opts.Extract(); err != nil {
			return errors.Wrapf(err, "problem extracting file %s to path %s", opts.Path, opts.ArchiveOpts.TargetPath)
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
			return errors.Errorf("could not detect archive format for %s", opts.Path)
		}
		var ok bool
		archiveHandler, ok = unzipper.(archiver.Unarchiver)
		if !ok {
			return errors.Errorf("%s was not a supported archive format [%T]", opts.Path, unzipper)
		}
	case ArchiveTarGz:
		archiveHandler = archiver.NewTarGz()
	case ArchiveZip:
		archiveHandler = archiver.NewZip()
	default:
		return errors.Errorf("unrecognized archive format %s", opts.ArchiveOpts.Format)
	}

	if err := archiveHandler.Unarchive(opts.Path, opts.ArchiveOpts.TargetPath); err != nil {
		return errors.Wrapf(err, "problem extracting archive %s to %s", opts.Path, opts.ArchiveOpts.TargetPath)
	}

	return nil
}
