package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/content"
	"github.com/kcodes0/decent/internal/protocol"
	"github.com/kcodes0/decent/internal/system"
)

type App struct {
	reader *bufio.Reader
	stdout io.Writer
	stderr io.Writer
	cwd    string
}

func Run(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	app := &App{
		reader: bufio.NewReader(os.Stdin),
		stdout: os.Stdout,
		stderr: os.Stderr,
		cwd:    cwd,
	}
	return app.run(args)
}

func (a *App) run(args []string) error {
	if len(args) == 0 {
		return a.usage()
	}

	switch args[0] {
	case "init":
		return a.initCmd()
	case "setup":
		return a.setupCmd()
	case "host":
		if len(args) < 2 {
			return fmt.Errorf("usage: decent host <repo>")
		}
		return a.hostCmd(args[1])
	case "status":
		return a.statusCmd()
	case "push":
		return a.pushCmd()
	case "help", "-h", "--help":
		return a.usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *App) usage() error {
	_, err := fmt.Fprintln(a.stdout, `decent - federated static hosting

Usage:
  decent init
  decent setup
  decent host <repo>
  decent status
  decent push`)
	return err
}

func (a *App) initCmd() error {
	if err := requireCommands("git", "gh"); err != nil {
		return err
	}

	repoRoot, err := a.ensureRepoRoot()
	if err != nil {
		return err
	}
	if _, err := system.RunShort(repoRoot, "gh", "auth", "status"); err != nil {
		return fmt.Errorf("gh auth status failed: %w", err)
	}

	defaultRepo := a.detectRemote(repoRoot)
	siteName, err := a.promptString("Site name", filepath.Base(repoRoot))
	if err != nil {
		return err
	}
	repoName, err := a.promptString("GitHub repo URL or owner/repo", defaultRepo)
	if err != nil {
		return err
	}
	region, err := a.promptString("Master region tag", "local")
	if err != nil {
		return err
	}
	masterSite, err := a.promptString("Master site URL", "http://127.0.0.1:8080")
	if err != nil {
		return err
	}
	masterAPI, err := a.promptString("Master API base URL", "http://127.0.0.1:8080")
	if err != nil {
		return err
	}

	if repoName == "" {
		createRepo, err := a.promptBool("Create a GitHub repo with gh", true)
		if err != nil {
			return err
		}
		if createRepo {
			repoName, err = a.promptString("GitHub repo name", siteName)
			if err != nil {
				return err
			}
			if err := runAttached(repoRoot, "gh", "repo", "create", repoName, "--source", ".", "--remote", "origin", "--public"); err != nil {
				return err
			}
			repoName = normalizeRepoInput(repoName)
		}
	}

	hash, err := content.HashTree(repoRoot, config.ManifestFileName)
	if err != nil {
		return err
	}

	manifest := &protocol.Manifest{
		Version:     "v0",
		SiteName:    siteName,
		Repo:        repoName,
		ContentHash: hash,
		UpdatedAt:   time.Now().UTC(),
		Master: protocol.MasterNode{
			ID:          "master",
			Region:      region,
			APIBaseURL:  strings.TrimRight(masterAPI, "/"),
			SiteBaseURL: strings.TrimRight(masterSite, "/"),
		},
		Nodes: []protocol.RegisteredNode{
			{
				ID:          "master",
				Role:        "master",
				Region:      region,
				PublicURL:   strings.TrimRight(masterSite, "/"),
				Status:      "healthy",
				ContentHash: hash,
				LastSeenAt:  time.Now().UTC(),
			},
		},
	}
	if err := config.WriteManifest(repoRoot, manifest); err != nil {
		return err
	}

	localCfg := defaultConfig("master")
	localCfg.NodeID = "master"
	localCfg.Region = region
	localCfg.Repo = repoName
	localCfg.RepoDir = repoRoot
	localCfg.SiteDir = repoRoot
	localCfg.MasterAPI = manifest.Master.APIBaseURL
	localCfg.MasterSite = manifest.Master.SiteBaseURL
	localCfg.PublicHost, localCfg.PublicPort = splitHostPort(manifest.Master.SiteBaseURL, "127.0.0.1", 8080)
	localCfg.AdminPort = localCfg.PublicPort
	if err := config.WriteLocalConfig(localCfg); err != nil {
		return err
	}

	added := false
	if err := runAttached(repoRoot, "git", "add", config.ManifestFileName); err == nil {
		added = true
	}
	commitNow, err := a.promptBool("Commit manifest now", true)
	if err != nil {
		return err
	}
	if commitNow && added {
		if err := commitIfNeeded(repoRoot, "Initialize decent manifest"); err != nil {
			return err
		}
		if hasRemote(repoRoot) {
			if err := runAttached(repoRoot, "git", "push"); err != nil {
				return err
			}
		}
	}

	_, _ = fmt.Fprintf(a.stdout, "Wrote %s and local config.\nRun `decent-node --config %s` on the master to serve and route traffic.\n", filepath.Join(repoRoot, config.ManifestFileName), mustLocalConfigPath())
	return nil
}

func (a *App) setupCmd() error {
	cfg := defaultConfig("worker")
	if existing, err := config.ReadLocalConfig(); err == nil && existing != nil {
		cfg = existing
	}

	var err error
	cfg.Role = "worker"
	cfg.Region, err = a.promptString("Region/location tag", cfg.Region)
	if err != nil {
		return err
	}
	cfg.PublicHost, err = a.promptString("Public host to bind/advertise", cfg.PublicHost)
	if err != nil {
		return err
	}
	cfg.PublicPort, err = a.promptInt("Public port", cfg.PublicPort)
	if err != nil {
		return err
	}
	cfg.AdminPort, err = a.promptInt("Admin port", cfg.AdminPort)
	if err != nil {
		return err
	}
	cfg.MasterAPI, err = a.promptString("Master API base URL", cfg.MasterAPI)
	if err != nil {
		return err
	}
	cfg.MasterSite, err = a.promptString("Master site URL", cfg.MasterSite)
	if err != nil {
		return err
	}
	cfg.MaxBandwidthMbps, err = a.promptInt("Bandwidth budget Mbps", cfg.MaxBandwidthMbps)
	if err != nil {
		return err
	}
	cfg.MaxStorageMB, err = a.promptInt("Storage budget MB", cfg.MaxStorageMB)
	if err != nil {
		return err
	}
	syncSeconds, err := a.promptInt("Sync interval seconds", int(cfg.SyncInterval/time.Second))
	if err != nil {
		return err
	}
	heartbeatSeconds, err := a.promptInt("Heartbeat interval seconds", int(cfg.HeartbeatInterval/time.Second))
	if err != nil {
		return err
	}
	cfg.SyncInterval = time.Duration(syncSeconds) * time.Second
	cfg.HeartbeatInterval = time.Duration(heartbeatSeconds) * time.Second

	if err := config.WriteLocalConfig(cfg); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.stdout, "Saved worker config to %s\n", mustLocalConfigPath())
	return nil
}

func (a *App) hostCmd(repoArg string) error {
	if err := requireCommands("git"); err != nil {
		return err
	}

	cfg := defaultConfig("worker")
	if existing, err := config.ReadLocalConfig(); err == nil && existing != nil {
		cfg = existing
	}

	repoURL := normalizeRepoInput(repoArg)
	if repoURL == "" {
		return fmt.Errorf("invalid repo %q", repoArg)
	}

	sitesDir, err := config.StateDir()
	if err != nil {
		return err
	}
	cloneDir := filepath.Join(filepath.Dir(sitesDir), "sites", repoSlug(repoArg))
	if err := os.MkdirAll(filepath.Dir(cloneDir), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := runAttached("", "git", "clone", repoURL, cloneDir); err != nil {
			return err
		}
	}

	manifest, err := config.ReadManifest(cloneDir)
	if err != nil {
		return err
	}
	if err := verifyRepoHash(cloneDir, manifest.ContentHash); err != nil {
		return err
	}

	cfg.Role = "worker"
	cfg.Repo = repoURL
	cfg.RepoDir = cloneDir
	cfg.SiteDir = cloneDir
	cfg.MasterAPI = strings.TrimRight(manifest.Master.APIBaseURL, "/")
	cfg.MasterSite = strings.TrimRight(manifest.Master.SiteBaseURL, "/")
	if err := config.WriteLocalConfig(cfg); err != nil {
		return err
	}

	if err := a.startDaemonDetached(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(a.stdout, "Worker hosting %s from %s\n", manifest.SiteName, cloneDir)
	return nil
}

func (a *App) statusCmd() error {
	cfg, err := config.ReadLocalConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no local config found, run `decent init` or `decent setup` first")
	}

	_, _ = fmt.Fprintf(a.stdout, "Local node: %s (%s)\n", cfg.NodeID, cfg.Role)
	_, _ = fmt.Fprintf(a.stdout, "Region: %s\n", cfg.Region)
	if cfg.Repo != "" {
		_, _ = fmt.Fprintf(a.stdout, "Repo: %s\n", cfg.Repo)
	}

	adminURL := localAdminURL(cfg)
	if adminURL != "" {
		if body, err := getJSON(adminURL, "/status"); err == nil {
			_, _ = fmt.Fprintf(a.stdout, "\nLocal daemon:\n%s\n", indent(body))
		}
	}

	if cfg.MasterAPI != "" {
		if body, err := getJSON(strings.TrimRight(cfg.MasterAPI, "/"), "/api/status"); err == nil {
			_, _ = fmt.Fprintf(a.stdout, "\nNetwork:\n%s\n", indent(body))
		}
	}
	return nil
}

func (a *App) pushCmd() error {
	if err := requireCommands("git"); err != nil {
		return err
	}
	repoRoot, err := a.ensureRepoRoot()
	if err != nil {
		return err
	}

	manifest, err := config.ReadManifest(repoRoot)
	if err != nil {
		return err
	}
	hash, err := content.HashTree(repoRoot, config.ManifestFileName)
	if err != nil {
		return err
	}
	manifest.ContentHash = hash
	manifest.UpdatedAt = time.Now().UTC()

	if manifest.Master.APIBaseURL != "" {
		if status, err := fetchStatus(strings.TrimRight(manifest.Master.APIBaseURL, "/") + "/api/status"); err == nil && status != nil {
			manifest.Nodes = append([]protocol.RegisteredNode{{
				ID:          manifest.Master.ID,
				Role:        "master",
				Region:      manifest.Master.Region,
				PublicURL:   manifest.Master.SiteBaseURL,
				Status:      "healthy",
				ContentHash: manifest.ContentHash,
				LastSeenAt:  time.Now().UTC(),
			}}, status.KnownNodes...)
		}
	}

	if err := config.WriteManifest(repoRoot, manifest); err != nil {
		return err
	}
	if err := runAttached(repoRoot, "git", "add", "-A"); err != nil {
		return err
	}
	if err := commitIfNeeded(repoRoot, "Update decent content hash"); err != nil {
		return err
	}
	if hasRemote(repoRoot) {
		if err := runAttached(repoRoot, "git", "push"); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(a.stdout, "Updated %s and pushed the repo\n", config.ManifestFileName)
	return nil
}

func (a *App) ensureRepoRoot() (string, error) {
	root, err := system.RunShort(a.cwd, "git", "rev-parse", "--show-toplevel")
	if err == nil {
		return root, nil
	}
	initRepo, promptErr := a.promptBool("Current directory is not a git repo. Run `git init` here", true)
	if promptErr != nil {
		return "", promptErr
	}
	if !initRepo {
		return "", fmt.Errorf("git repository required")
	}
	if err := runAttached(a.cwd, "git", "init"); err != nil {
		return "", err
	}
	return a.cwd, nil
}

func (a *App) detectRemote(repoRoot string) string {
	out, err := system.RunShort(repoRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (a *App) promptString(label, def string) (string, error) {
	if def != "" {
		_, _ = fmt.Fprintf(a.stdout, "%s [%s]: ", label, def)
	} else {
		_, _ = fmt.Fprintf(a.stdout, "%s: ", label)
	}
	line, err := a.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func (a *App) promptInt(label string, def int) (int, error) {
	value, err := a.promptString(label, strconv.Itoa(def))
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", label)
	}
	return n, nil
}

func (a *App) promptBool(label string, def bool) (bool, error) {
	defaultText := "y"
	if !def {
		defaultText = "n"
	}
	value, err := a.promptString(label+" (y/n)", defaultText)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return def, nil
	}
}

func startCommand(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	return cmd, nil
}

func (a *App) startDaemonDetached() error {
	cfgPath, err := config.LocalConfigPath()
	if err != nil {
		return err
	}
	logPath, err := config.LogFilePath()
	if err != nil {
		return err
	}
	pidPath, err := config.PidFilePath()
	if err != nil {
		return err
	}
	if err := config.EnsureDir(filepath.Dir(logPath)); err != nil {
		return err
	}

	nodeBin, err := exec.LookPath("decent-node")
	if err != nil {
		return fmt.Errorf("decent-node binary not found in PATH")
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	cmd, err := startCommand(nodeBin, "--config", cfgPath)
	if err != nil {
		_ = logFile.Close()
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = cmd.Process.Release()
	_ = logFile.Close()
	_, _ = fmt.Fprintf(a.stdout, "Started decent-node (pid %d). Logs: %s\n", cmd.Process.Pid, logPath)
	return nil
}

func verifyRepoHash(repoDir string, expected string) error {
	actual, err := content.HashTree(repoDir, config.ManifestFileName)
	if err != nil {
		return err
	}
	if actual == expected {
		return nil
	}
	if err := runAttached(repoDir, "git", "reset", "--hard", "HEAD"); err != nil {
		return err
	}
	if err := runAttached(repoDir, "git", "clean", "-fd"); err != nil {
		return err
	}
	actual, err = content.HashTree(repoDir, config.ManifestFileName)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("content hash mismatch: got %s want %s", actual, expected)
	}
	return nil
}

func requireCommands(names ...string) error {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("required command %q not found in PATH", name)
		}
	}
	return nil
}

func runAttached(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func commitIfNeeded(repoRoot, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return nil
	}
	if err := runAttached(repoRoot, "git", "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

func hasRemote(repoRoot string) bool {
	out, err := system.RunShort(repoRoot, "git", "remote")
	return err == nil && strings.TrimSpace(out) != ""
}

func getJSON(baseURL, path string) (string, error) {
	resp, err := http.Get(strings.TrimRight(baseURL, "/") + path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}
	return string(data), nil
}

func fetchStatus(rawURL string) (*protocol.StatusResponse, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("master status failed: %s", resp.Status)
	}
	var out protocol.StatusResponse
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeJSON(r io.Reader, out any) error {
	return json.NewDecoder(r).Decode(out)
}

func localAdminURL(cfg *protocol.LocalConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.Role == "master" && cfg.MasterAPI != "" {
		return strings.TrimRight(cfg.MasterAPI, "/")
	}
	if cfg.AdminPort == 0 {
		return ""
	}
	host := cfg.PublicHost
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.AdminPort)
}

func defaultConfig(role string) *protocol.LocalConfig {
	return &protocol.LocalConfig{
		NodeID:            fmt.Sprintf("%s-%d", role, time.Now().UnixNano()),
		Role:              role,
		Region:            "local",
		PublicHost:        "127.0.0.1",
		PublicPort:        8081,
		AdminPort:         8082,
		MasterAPI:         "http://127.0.0.1:8080",
		MasterSite:        "http://127.0.0.1:8080",
		MaxBandwidthMbps:  100,
		MaxStorageMB:      10240,
		SyncInterval:      60 * time.Second,
		HeartbeatInterval: 30 * time.Second,
		RedirectMode:      "redirect",
	}
}

func normalizeRepoInput(repo string) string {
	repo = strings.TrimSpace(repo)
	switch {
	case repo == "":
		return ""
	case strings.HasPrefix(repo, "github:"):
		return "https://github.com/" + strings.TrimPrefix(repo, "github:") + ".git"
	case strings.HasPrefix(repo, "http://"), strings.HasPrefix(repo, "https://"), strings.HasPrefix(repo, "git@"):
		return repo
	case strings.Count(repo, "/") == 1:
		return "https://github.com/" + repo + ".git"
	default:
		return repo
	}
}

func repoSlug(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimPrefix(repo, "github:")
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.ReplaceAll(repo, "/", "-")
	if repo == "" {
		return "site"
	}
	return repo
}

func splitHostPort(rawURL, defaultHost string, defaultPort int) (string, int) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return defaultHost, defaultPort
	}
	host := u.Hostname()
	port := defaultPort
	if u.Port() != "" {
		if parsed, err := strconv.Atoi(u.Port()); err == nil {
			port = parsed
		}
	}
	if host == "" {
		host = defaultHost
	}
	return host, port
}

func indent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func mustLocalConfigPath() string {
	path, err := config.LocalConfigPath()
	if err != nil {
		return "~/.config/decent/node.toml"
	}
	return path
}
