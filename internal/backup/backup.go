package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"update-ai-tools/internal/platform"
	"update-ai-tools/internal/redactor"
	"update-ai-tools/internal/report"
)

func Configs(profile platform.Profile, red redactor.Redactor, log *report.Logger) (string, report.TaskResult) {
	start := time.Now()
	dest := filepath.Join(profile.CodexHome, "backups", "update-ai-tools", start.Format("20060102-150405"))
	res := report.TaskResult{Name: "backup-configs", Provider: "backup"}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		res.Status = report.StatusFailed
		res.Summary = "create backup directory failed"
		res.Error = err.Error()
		return dest, res
	}
	copied := 0
	var firstErr error
	for _, src := range profile.ConfigFiles {
		if _, err := os.Stat(src); err != nil {
			continue
		}
		target := filepath.Join(dest, safeName(src))
		if err := copyFile(src, target); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Detailf("backup failed %s: %v", red.Redact(src), err)
			continue
		}
		copied++
	}
	res.Duration = time.Since(start)
	if firstErr != nil {
		if copied == 0 {
			res.Status = report.StatusFailed
			res.Summary = "failed to back up any configs"
		} else {
			res.Status = report.StatusWarning
			res.Summary = fmt.Sprintf("backed up %d configs; some failed", copied)
		}
		res.Error = firstErr.Error()
		return dest, res
	}
	res.Status = report.StatusSuccess
	res.Summary = fmt.Sprintf("backed up %d configs", copied)
	return dest, res
}

func safeName(path string) string {
	clean := filepath.Clean(path)
	vol := filepath.VolumeName(clean)
	clean = clean[len(vol):]
	out := ""
	if vol != "" {
		out = strings.ReplaceAll(vol, ":", "__")
	}
	for _, r := range clean {
		switch r {
		case '/', '\\', ':':
			out += "__"
		default:
			out += string(r)
		}
	}
	return out
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
