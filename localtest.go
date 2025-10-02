//go:generate go run ./helper/manifest

package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const appName = "localtest"
const appUrl = "https://local.test/"

var (
	buildVersion = "unknown"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

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

type VersionInfo struct {
	Version [60]byte
	Commit  [40]byte
	Date    [20]byte
}

func writeVersionFile(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", cerr)
		}
	}()

	info := VersionInfo{}

	// Copy with truncation/padding
	copy(info.Version[:], []byte(buildVersion))
	copy(info.Commit[:], []byte(buildCommit))
	copy(info.Date[:], []byte(buildDate))

	if err := binary.Write(file, binary.LittleEndian, info); err != nil {
		return err
	}

	return nil
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

func initStack() error {
	dest := stackDir()

	versionFile := filepath.Join(dest, ".version")

	if !fileExists(versionFile) {
		if err := extractStackFiles(dest); err != nil {
			return err
		}

		if err := writeVersionFile(versionFile); err != nil {
			return err
		}
	}

	if err := injectLocalRootCA(dest); err != nil {
		return err
	}

	return nil
}

func verifyAllSHA256(dest string) error {
	fmt.Printf("Verifying integrity of %s stack ...\n", appName)

	failures := 0

	for _, file := range fileList {
		dst := filepath.Join(dest, file.Path)

		valid, err := verifySHA256(dst, file.Sha256)
		if err != nil {
			return err
		}

		if valid {
			fmt.Printf("%s: OK\n", file.Path)
		} else {
			fmt.Printf("%s: FAILED\n", file.Path)
			failures++
		}
	}

	if failures > 0 {
		return fmt.Errorf("WARNING: %d computed checksum did NOT match", failures)
	}

	return nil
}

func extractStackFiles(dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("failed to remove stack dir: %w", err)
	}

	fmt.Printf("Extracting %d files to %s directory:\n", len(fileList), dest)
	for _, file := range fileList {
		data, err := embeddedFiles.ReadFile(file.Path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dest, file.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		fmt.Printf("- %s (%.1fK)\n", file.Path, float64(file.Size)/1024)
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

var showDir bool

func init() {
	rootCmd.AddCommand(upCmd, downCmd, logsCmd, psCmd, rmCmd, verifyCmd)
	rootCmd.Flags().BoolVar(&showDir, "dir", false, "Print stack directory and exit")

	// disable the '-v' alias for the version flag
	rootCmd.Flags().Bool("version", false, "Print version information and exit")
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
}

var rootCmd = &cobra.Command{
	Use:          appName,
	Short:        fmt.Sprintf("Smart %s stack controller", appName),
	SilenceUsage: true,
	Version:      renderVersion(),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if showDir {
			fmt.Println(stackDir())
			os.Exit(0)
		}
		return nil
	},
}

var upCmd = &cobra.Command{
	Use:                "up",
	Short:              "Spin up the stack",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initStack(); err != nil {
			return err
		}
		if err := runDockerCompose(append([]string{"up", "--wait"}, args...)...); err != nil {
			return err
		}
		fmt.Printf("\nNow it's time to open %s and enjoy local development. ;)\n", appUrl)
		return nil
	},
}

var downCmd = &cobra.Command{
	Use:                "down",
	Short:              "Tear down the stack",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerCompose(append([]string{"down", "--remove-orphans"}, args...)...)
	},
}

var logsCmd = &cobra.Command{
	Use:                "logs",
	Short:              "Show stack logs",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerCompose(append([]string{"logs"}, args...)...)
	},
}

var psCmd = &cobra.Command{
	Use:                "ps",
	Short:              "Show stack containers",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDockerCompose(append([]string{"ps"}, args...)...)
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Tear down the stack and remove cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = runDockerCompose("down", "--remove-orphans", "--volumes", "--rmi", "all")
		fmt.Println("Stack cleaned up")

		if err := os.RemoveAll(stackDir()); err != nil {
			return fmt.Errorf("failed to remove stack dir: %w", err)
		}
		fmt.Println("Storage cleaned up")
		return nil
	},
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify integrity",
	RunE: func(cmd *cobra.Command, args []string) error {
		return verifyAllSHA256(stackDir())
	},
}

func runDockerCompose(args ...string) error {
	dest := stackDir()
	composeFile := filepath.Join(dest, "compose.yaml")

	if _, err := os.Stat(composeFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("No compose.yaml found in %s", dest)
		}
		return err
	}

	cmdArgs := append([]string{"compose", "-f", composeFile}, args...)

	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	preview_cmd := strings.Join(append([]string{"docker", "compose"}, args...), " ")

	fmt.Printf("Running %q command ...\n", preview_cmd)
	return cmd.Run()
}
