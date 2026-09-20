package automation

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// backupConfig is the shape of the "backup" automation rule's config JSON.
type backupConfig struct {
	SourcePaths []string `json:"sourcePaths"`
	DestDir     string   `json:"destDir"`
	Retain      int      `json:"retain"`
}

func (r *Runner) runBackup(ctx context.Context, cfg backupConfig) {
	if cfg.DestDir == "" || len(cfg.SourcePaths) == 0 {
		log.Printf("automation: backup skipped, no sourcePaths/destDir configured")
		return
	}
	if err := os.MkdirAll(cfg.DestDir, 0o755); err != nil {
		log.Printf("automation: backup: create dest dir: %v", err)
		return
	}

	retain := cfg.Retain
	if retain <= 0 {
		retain = 7
	}

	for _, src := range cfg.SourcePaths {
		if err := backupOne(ctx, src, cfg.DestDir); err != nil {
			log.Printf("automation: backup of %s failed: %v", src, err)
			continue
		}
		if err := pruneOldBackups(cfg.DestDir, backupPrefix(src), retain); err != nil {
			log.Printf("automation: backup retention cleanup for %s failed: %v", src, err)
		}
	}
}

func backupPrefix(sourcePath string) string {
	return strings.TrimSuffix(filepath.Base(sourcePath), string(filepath.Separator))
}

func backupOne(ctx context.Context, sourcePath, destDir string) error {
	name := fmt.Sprintf("%s-%s.tar.gz", backupPrefix(sourcePath), time.Now().UTC().Format("20060102-150405"))
	destPath := filepath.Join(destDir, name)

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
}

// pruneOldBackups keeps only the `retain` most recent archives for a given
// source's prefix, deleting the rest.
func pruneOldBackups(destDir, prefix string, retain int) error {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix+"-") && strings.HasSuffix(e.Name(), ".tar.gz") {
			matches = append(matches, e.Name())
		}
	}
	sort.Strings(matches) // timestamp-suffixed names sort chronologically

	if len(matches) <= retain {
		return nil
	}
	for _, name := range matches[:len(matches)-retain] {
		if err := os.Remove(filepath.Join(destDir, name)); err != nil {
			return err
		}
	}
	return nil
}
