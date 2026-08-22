package shared

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	sysRuntime "runtime"
	"strings"
	"time"
)

func Md5(data string) string {
	hashNew := md5.New()
	hashNew.Write([]byte(data))
	hash := hashNew.Sum(nil)
	return hex.EncodeToString(hash)
}

func FormatSize(size float64) string {
	if size > 1048576 {
		return fmt.Sprintf("%.2fMB", float64(size)/1048576)
	}
	if size > 1024 {
		return fmt.Sprintf("%.2fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%.0fb", size)
}

func GetTopLevelDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Host != "" {
		rawURL = u.Host
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(rawURL)
	if err != nil {
		return rawURL
	}
	return domain
}

func FileExist(file string) bool {
	info, err := os.Stat(file)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func CreateDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0750)
	}
	return nil
}

func IsDevelopment() bool {
	return os.Getenv("APP_ENV") == "development"
}

func GetFileNameFromURL(rawUrl string) string {
	parsedURL, err := url.Parse(rawUrl)
	if err != nil {
		return ""
	}

	fileName := path.Base(parsedURL.Path)
	if fileName == "" || fileName == "/" {
		return ""
	}

	if decoded, err := url.QueryUnescape(fileName); err == nil {
		fileName = decoded
	}

	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	fileName = re.ReplaceAllString(fileName, "_")

	fileName = strings.TrimRightFunc(fileName, func(r rune) bool {
		return r == '.' || r == ' '
	})

	const maxFileNameLen = 255
	runes := []rune(fileName)
	if len(runes) > maxFileNameLen {
		ext := path.Ext(fileName)
		name := strings.TrimSuffix(fileName, ext)

		runes = []rune(name)
		if len(runes) > maxFileNameLen-len(ext) {
			runes = runes[:maxFileNameLen-len(ext)]
		}
		name = string(runes)
		fileName = name + ext
	}

	return fileName
}

func SanitizeFileName(name string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*\r\n\t]`)
	cleaned := re.ReplaceAllString(name, "_")
	cleaned = strings.TrimFunc(cleaned, func(r rune) bool {
		return r == '.' || r == ' '
	})
	if cleaned == "" {
		cleaned = "media"
	}
	return cleaned
}

func GetCurrentDateTimeFormatted() string {
	now := time.Now()
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d",
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		now.Minute(),
		now.Second())
}

func GetUniqueFileName(filePath string) string {
	if !FileExist(filePath) {
		return filePath
	}

	ext := filepath.Ext(filePath)
	baseName := strings.TrimSuffix(filePath, ext)
	count := 1

	for {
		newFileName := fmt.Sprintf("%s(%d)%s", baseName, count, ext)
		if !FileExist(newFileName) {
			return newFileName
		}
		count++
	}
}

func OpenFolder(filePath string) error {
	var cmd *exec.Cmd

	switch sysRuntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	case "windows":
		absPath, _ := filepath.Abs(filePath)
		absPath = filepath.FromSlash(absPath)
		if fi, err := os.Stat(absPath); err == nil && !fi.IsDir() {
			// Windows explorer /select,<file> opens folder and selects the file
			cmd = exec.Command("explorer", "/select,", absPath)
		} else {
			dir := absPath
			if fi != nil && !fi.IsDir() {
				dir = filepath.Dir(absPath)
			} else if _, err := os.Stat(absPath); err != nil {
				dir = filepath.Dir(absPath)
			}
			cmd = exec.Command("explorer", dir)
		}
	case "linux":
		if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
			cmd = exec.Command("nautilus", "--select", filePath)
		} else {
			cmd = exec.Command("nautilus", filePath)
		}
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("thunar", filePath)
			if err := cmd.Start(); err != nil {
				cmd = exec.Command("dolphin", filePath)
				if err := cmd.Start(); err != nil {
					cmd = exec.Command("pcmanfm", filePath)
					if err := cmd.Start(); err != nil {
						return err
					}
				}
			}
		}
	default:
		return errors.New("unsupported platform")
	}

	return cmd.Start()
}

func FindFFmpegPath() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}

	exePath, err := os.Executable()
	exeDir := ""
	if err == nil {
		exeDir = filepath.Dir(exePath)
	}

	candidates := []string{
		"./ffmpeg/ffmpeg.exe",
		"./ffmpeg.exe",
		"./bin/ffmpeg.exe",
		"ffmpeg/ffmpeg.exe",
		"ffmpeg.exe",
		filepath.Join(exeDir, "ffmpeg", "ffmpeg.exe"),
		filepath.Join(exeDir, "ffmpeg.exe"),
		filepath.Join(exeDir, "bin", "ffmpeg.exe"),
		filepath.Join(exeDir, "..", "ffmpeg", "ffmpeg.exe"),
	}

	for _, c := range candidates {
		if c != "" && FileExist(c) {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}

func MergeMediaWithFFmpeg(videoPath, audioPath, outputPath string) error {
	ffmpegPath := FindFFmpegPath()
	if ffmpegPath == "" {
		return errors.New("ffmpeg not found in PATH or local directory")
	}

	cmd := exec.Command(ffmpegPath, "-i", videoPath, "-i", audioPath, "-c", "copy", "-y", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg merge error: %v, output: %s", err, string(output))
	}
	return nil
}
