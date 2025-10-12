//go:generate go run ./internal/manifest

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const appName = "localtest"
const appUrl = "https://local.test/"

const stackInfoFile = ".localtest"
const stackRebuildFile = ".rebuild"

var (
	buildVersion = "unknown"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

var ErrStackNotExist = errors.New("docker compose stack does not exist")
var ErrMagicHeaderInvalid = errors.New("magic header invalid")

func stackDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	// mimic $XDG_CACHE_HOME
	return filepath.Join(home, ".cache", appName)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return !info.IsDir()
}

type BinaryVersion struct {
	Version string
	Commit  string
	Date    string
}

type StackVersion struct {
	SpecVersion uint8
	Binary      BinaryVersion
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewStackVersionV1() *StackVersion {
	now := time.Now().Local()

	sv := &StackVersion{
		SpecVersion: 1,
		Binary: BinaryVersion{
			Version: buildVersion,
			Commit:  buildCommit,
			Date:    buildDate,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	return sv
}

func (sv *StackVersion) RecordUpdate() {
	sv.UpdatedAt = time.Now().Local()

	if sv.Binary.Version != buildVersion {
		sv.Binary.Version = buildVersion
		sv.Binary.Commit = buildCommit
		sv.Binary.Date = buildDate
	}
}

func (sv StackVersion) IsSameBinaryVersion() bool {
	return sv.Binary.Version == buildVersion
}

func (sv StackVersion) SaveToFile(path string) error {
	data, err := sv.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp file error: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file error: %w", err)
	}

	return nil
}

func (sv *StackVersion) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	return sv.UnmarshalBinary(data)
}

// Magic header for content negotiation/versioning
var stackVersionMagicHeader = []byte("SVMH")

func (s StackVersion) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)

	if _, err := buf.Write(stackVersionMagicHeader); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, s.SpecVersion); err != nil {
		return nil, err
	}

	writeString := func(str string) error {
		b := []byte(str)
		length := int32(len(b))
		if err := binary.Write(buf, binary.BigEndian, length); err != nil {
			return err
		}
		if err := binary.Write(buf, binary.BigEndian, b); err != nil {
			return err
		}
		return nil
	}

	writeTime := func(t time.Time) error {
		ts := t.Unix()
		return binary.Write(buf, binary.BigEndian, ts)
	}

	if err := writeString(s.Binary.Version); err != nil {
		return nil, err
	}
	if err := writeString(s.Binary.Commit); err != nil {
		return nil, err
	}
	if err := writeString(s.Binary.Date); err != nil {
		return nil, err
	}

	if err := writeTime(s.CreatedAt); err != nil {
		return nil, err
	}
	if err := writeTime(s.UpdatedAt); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *StackVersion) UnmarshalBinary(data []byte) error {
	if len(data) < len(stackVersionMagicHeader)+1 {
		return fmt.Errorf("data too short (%w)", ErrMagicHeaderInvalid)
	}

	header := data[:len(stackVersionMagicHeader)]
	payload := data[len(stackVersionMagicHeader):]

	if !bytes.Equal(header, stackVersionMagicHeader) {
		return ErrMagicHeaderInvalid
	}

	r := bytes.NewReader(payload)

	readString := func() (string, error) {
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return "", err
		}
		if length < 0 {
			return "", fmt.Errorf("invalid string length: %d", length)
		}
		b := make([]byte, length)
		if _, err := io.ReadFull(r, b); err != nil {
			return "", err
		}
		return string(b), nil
	}

	readTime := func() (time.Time, error) {
		var ts int64
		if err := binary.Read(r, binary.BigEndian, &ts); err != nil {
			return time.Time{}, err
		}
		return time.Unix(ts, 0).Local(), nil
	}

	spec, err := r.ReadByte()
	if err != nil {
		return err
	}
	s.SpecVersion = spec

	if s.Binary.Version, err = readString(); err != nil {
		return err
	}
	if s.Binary.Commit, err = readString(); err != nil {
		return err
	}
	if s.Binary.Date, err = readString(); err != nil {
		return err
	}
	if s.CreatedAt, err = readTime(); err != nil {
		return err
	}
	if s.UpdatedAt, err = readTime(); err != nil {
		return err
	}

	return nil
}

func Confirm(prompt string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)

	if defaultYes {
		fmt.Printf("%s [Y/n]: ", prompt)
	} else {
		fmt.Printf("%s [y/N]: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return (defaultYes && input == "") || input == "y" || input == "yes"
}

func HasInternetConnection() bool {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Head("https://www.google.com/generate_204")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNoContent
}

func computeSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", cerr)
		}
	}()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), err
}

func verifySHA256(filePath, expectedHashHex string) (bool, error) {
	computedHashHex, err := computeSHA256(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to compute hash: %w", err)
	}

	expectedHashBytes, err := hex.DecodeString(expectedHashHex)
	if err != nil {
		return false, fmt.Errorf("invalid expected hash format: %w", err)
	}

	computedHashBytes, err := hex.DecodeString(computedHashHex)
	if err != nil {
		return false, fmt.Errorf("invalid computed hash format: %w", err)
	}

	if len(expectedHashBytes) != sha256.Size || len(computedHashBytes) != sha256.Size {
		return false, nil
	}

	hashesMatch := subtle.ConstantTimeCompare(expectedHashBytes, computedHashBytes) == 1

	return hashesMatch, nil
}

func copyFile(src, dst string) error {
	sfi, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if tfi, err := os.Stat(dst); err == nil {
		if tfi.Mode().Perm()&0200 == 0 {
			if err := os.Chmod(dst, 0600); err != nil {
				return err
			}
		}
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sfi.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

const chunkSize = 64000

func deepCompare(file1, file2 string) (bool, error) {
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	for {
		b1 := make([]byte, chunkSize)
		_, err1 := f1.Read(b1)

		b2 := make([]byte, chunkSize)
		_, err2 := f2.Read(b2)

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return true, nil
			}
			if err1 == io.EOF || err2 == io.EOF {
				return false, nil
			}
			if err1 != nil {
				return false, err1
			}
			return false, err2
		}

		if !bytes.Equal(b1, b2) {
			return false, nil
		}
	}
}

func syncStack(update bool) (bool, error) {
	dest := stackDir()
	infoFile := filepath.Join(dest, stackInfoFile)
	rebuildFile := filepath.Join(dest, stackRebuildFile)

	sv := NewStackVersionV1()

	if err := sv.LoadFromFile(infoFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrMagicHeaderInvalid) {
			return false, fmt.Errorf("stack exists, failed to read info (%w)\n", err)
		}

		update = true
	}

	rebuild := fileExists(rebuildFile)

	if update {
		if err := extractStackFiles(dest, true); err != nil {
			return false, err
		}

		if err := injectLocalRootCA(dest); err != nil {
			return false, err
		}

		rebuild = rebuild || !sv.IsSameBinaryVersion()

		sv.RecordUpdate()

		if err := sv.SaveToFile(infoFile); err != nil {
			return false, fmt.Errorf("failed saving, err: %w\n", err)
		}

		if rebuild {
			os.WriteFile(rebuildFile, []byte{}, 0600)
		}
	}

	return rebuild, nil
}

func verifyAllSHA256(dest string) error {
	fmt.Printf("Verifying integrity of %s stack ...\n", appName)

	failures := 0

	for _, file := range stackFilesMeta {
		dst := filepath.Join(dest, file.Path)

		valid, err := verifySHA256(dst, file.Sha256)
		if err != nil {
			return err
		}

		if !valid {
			fmt.Printf("%s: FAILED\n", file.Path)
			failures++
		}
	}

	if failures > 0 {
		return fmt.Errorf("WARNING: %d computed checksum did NOT match", failures)
	}

	return nil
}

func showInfo(dest string) error {
	fmt.Printf("Stack directory:   %s\n", dest)

	infoFile := filepath.Join(dest, stackInfoFile)
	rebuildFile := filepath.Join(dest, stackRebuildFile)

	var sv StackVersion

	if err := sv.LoadFromFile(infoFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no stack found, run 'sync' first.\n")
		}
		return fmt.Errorf("stack exists, failed to read info (%w)\n", err)
	}

	rebuild := fileExists(rebuildFile)

	fmt.Printf("Stack created at:  %s (%s ago)\n", sv.CreatedAt.Format(time.RFC3339), time.Since(sv.CreatedAt).Round(time.Second))
	fmt.Printf("Stack updated at:  %s (%s ago)\n", sv.UpdatedAt.Format(time.RFC3339), time.Since(sv.UpdatedAt).Round(time.Second))
	fmt.Printf("Stack version:     %s (Built on %s from %s commit)\n", sv.Binary.Version, sv.Binary.Commit, sv.Binary.Date)
	fmt.Printf("Binary version:    %s (Built on %s from %s commit)\n", buildVersion, buildCommit, buildDate)

	if !sv.IsSameBinaryVersion() {
		fmt.Printf("\nINFO: stack and binary versions do not match, please run 'sync' command.\n")
	}

	if rebuild {
		fmt.Printf("\nINFO: stack rebuild is pending, please run 'up' command.\n")
	}

	return nil
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	defer f.Close()

	if _, err = f.Readdirnames(1); err == io.EOF {
		return true, nil
	}
	return false, err
}

func extractStackFiles(dest string, cleanup bool) error {
	if cleanup {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("failed to remove stack dir: %w", err)
		}
	} else {
		isEmpty, err := isDirEmpty(dest)
		if err != nil {
			return err
		}

		if !isEmpty {
			return fmt.Errorf("Directory is not empty.")
		}
	}

	fmt.Printf("Extracting %d files to %s directory:\n", len(stackFilesMeta), dest)
	for _, file := range stackFilesMeta {
		data, err := stackFilesFS.ReadFile(file.Path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dest, file.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		fmt.Printf("%s %s (%.1fK)\n", file.Perm.String(), file.Path, float64(file.Size)/1024)
		if err := os.WriteFile(dst, data, file.Perm); err != nil {
			return err
		}

		valid, err := verifySHA256(dst, file.Sha256)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("Checksum for %q is", file.Path)
		}
	}

	if err := verifyAllSHA256(dest); err != nil {
		return err
	}

	return nil
}

func injectLocalRootCA(dest string) error {
	dest = filepath.Join(dest, "certs")

	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}

	out, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return fmt.Errorf("mkcert -CAROOT failed: %w", err)
	}
	caRoot := filepath.Clean(strings.TrimSpace(string(out)))

	files, err := filepath.Glob(filepath.Join(caRoot, "rootCA*.pem"))
	if err != nil {
		return err
	}

	if len(files) != 2 {
		return fmt.Errorf("No local Root CA found. MUST run 'mkcert -install' first")
	}

	var performSync = false

	for _, src := range files {
		dst := filepath.Join(dest, filepath.Base(src))

		isSame, err := deepCompare(src, dst)
		if err != nil || !isSame {
			performSync = true
		}
	}

	if !performSync {
		return nil
	}

	fmt.Printf("Copying local Root CA cert to %q directory ...\n", dest)

	for _, src := range files {
		dst := filepath.Join(dest, filepath.Base(src))

		fmt.Printf("- %s ...\n", src)

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %q to %q", src, dst)
		}
	}

	return nil
}

func renderVersion() string {
	return fmt.Sprintf("%s (Built on %s from %s commit)", buildVersion, buildDate, buildCommit)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cmdRm.Flags().BoolP("all", "a", false, "all resources such as volumes, images & etc ...")

	rootCmd.AddCommand(cmdSync, cmdRm, cmdVerify, cmdInfo)
	// docker compose proxies
	rootCmd.AddCommand(cmdUp, cmdDown, cmdLogs, cmdPs)

	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
}

var rootCmd = &cobra.Command{
	Use:          appName,
	Short:        fmt.Sprintf("Smart %s stack controller", appName),
	SilenceUsage: true,
	Version:      renderVersion(),
}

var cmdSync = &cobra.Command{
	Use:   "sync",
	Short: "Sync the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		rebuild, err := syncStack(true)

		if err != nil {
			return err
		}

		fmt.Printf("Synchronization DONE\n")

		if rebuild {
			fmt.Printf("\nINFO: stack rebuild is pending, please run 'up' command.\n")
		}

		return nil
	},
}

var cmdRm = &cobra.Command{
	Use:   "rm",
	Short: "Tear down the stack and remove cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		args = append([]string{"down", "--remove-orphans"}, args...)

		if cmd.Flags().Changed("all") {
			args = append(args, "--volumes", "--rmi", "all")
		}

		if err := runDockerCompose(args...); err != nil {
			if !errors.Is(err, ErrStackNotExist) {
				return err
			}

			fmt.Printf("Error: %v\n", err)
		}

		fmt.Println("Stack cleaned up")

		if err := os.RemoveAll(stackDir()); err != nil {
			return fmt.Errorf("failed to remove stack dir: %w", err)
		}

		fmt.Println("Storage cleaned up")

		return nil
	},
}

var cmdVerify = &cobra.Command{
	Use:   "verify",
	Short: "Verify integrity",
	RunE: func(cmd *cobra.Command, args []string) error {
		return verifyAllSHA256(stackDir())
	},
}

var cmdInfo = &cobra.Command{
	Use:   "info",
	Short: "Show info",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showInfo(stackDir())
	},
}

var cmdUp = &cobra.Command{
	Use:                "up",
	Short:              "Spin up the stack via 'docker compose up'",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		rebuild, err := syncStack(false)
		if err != nil {
			return err
		}

		dest := stackDir()
		rebuildFile := filepath.Join(dest, stackRebuildFile)

		args = append([]string{"up", "--wait", "--remove-orphans"}, args...)

		if rebuild {
			fmt.Printf("Stack upgrade is pending ...\n")
			if !HasInternetConnection() {
				fmt.Printf("WARN: running in offline mode, upgrade is postponed ;)\n")
				rebuild = false
			} else if !Confirm("Proceed with upgrade?", true) {
				fmt.Printf("INFO: decided to postpone upgrade\n")
				rebuild = false
			}
		}

		if rebuild {
			args = append(args, "--always-recreate-deps", "--force-recreate", "--build")
		}

		if err := runDockerCompose(args...); err != nil {
			return err
		}

		if rebuild {
			if err := os.RemoveAll(rebuildFile); err != nil {
				return err
			}
		}

		fmt.Printf("\nNow it's time to open %s and enjoy local development. ;)\n", appUrl)

		return nil
	},
}

var cmdDown = &cobra.Command{
	Use:                "down",
	Short:              "Tear down the stack via 'docker compose down'",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerCompose(append([]string{"down", "--remove-orphans"}, args...)...)
	},
}

var cmdLogs = &cobra.Command{
	Use:                "logs",
	Short:              "Show stack logs via 'docker compose logs'",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerCompose(append([]string{"logs"}, args...)...)
	},
}

var cmdPs = &cobra.Command{
	Use:                "ps",
	Short:              "Show stack containers via 'docker compose ps'",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		args = append([]string{"ps"}, args...)

		if err := runDockerCompose(args...); err != nil {
			return err
		}

		return nil
	},
}

func runDockerCompose(args ...string) error {
	dest := stackDir()
	composeFile := filepath.Join(dest, "compose.yaml")

	if _, err := os.Stat(composeFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w in %s", ErrStackNotExist, dest)
		}

		return err
	}

	cmdArgs := append([]string{"compose", "-f", composeFile}, args...)

	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	preview_cmd := strings.Join(append([]string{"docker", "compose"}, args...), " ")

	fmt.Printf("Proxying call to %q ...\n", preview_cmd)

	return cmd.Run()
}
