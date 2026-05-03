package auditfs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (r *Recorder) openAuditFile(forDate time.Time) (*os.File, error) {
	filePath := filepath.Join(
		r.root,
		fmt.Sprintf("%04d", forDate.Year()),
		fmt.Sprintf("%02d", int(forDate.Month())),
		fmt.Sprintf(
			"%04d_%02d_%02d_audit.jsonl",
			forDate.Year(),
			int(forDate.Month()),
			forDate.Day(),
		),
	)

	file, err := openAppendFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("open append file for date: %w", err)
	}

	return file, nil
}

func openAppendFile(filePath string) (*os.File, error) {
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0o700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) // #nosec G304 -- audit files are intentionally selected under the configured root.
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}

	return file, nil
}
