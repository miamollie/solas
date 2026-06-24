package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const ollamaHealthURL = "http://127.0.0.1:11434/api/tags"

func runCLI(args []string) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}

	switch args[0] {
	case "up":
		if err := ensureOllamaOnHost(); err != nil {
			fmt.Fprintf(os.Stderr, "ollama check failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "start Ollama on your host first, then retry `solas up`.")
			return true, 1
		}

		if err := runCompose(append([]string{"-p", "solas"}, "up", "-d", "--build")...); err != nil {
			fmt.Fprintf(os.Stderr, "failed to bring stack up: %v\n", err)
			return true, 1
		}

		fmt.Println("stack is up")
		fmt.Println("solas:      http://localhost:8000")
		fmt.Println("open-webui: http://localhost:3000")
		fmt.Println("prometheus: http://localhost:9090")
		fmt.Println("grafana:    http://localhost:3001 (admin/admin)")
		return true, 0
	case "down":
		if err := runCompose(append([]string{"-p", "solas"}, "down", "--remove-orphans")...); err != nil {
			fmt.Fprintf(os.Stderr, "failed to bring stack down: %v\n", err)
			return true, 1
		}
		fmt.Println("stack is down")
		return true, 0
	case "status":
		if err := runCompose(append([]string{"-p", "solas"}, "ps")...); err != nil {
			fmt.Fprintf(os.Stderr, "failed to read stack status: %v\n", err)
			return true, 1
		}

		if err := ensureOllamaOnHost(); err != nil {
			fmt.Fprintf(os.Stderr, "ollama(host): unavailable (%v)\n", err)
		} else {
			fmt.Println("ollama(host): healthy")
		}
		return true, 0
	case "logs":
		composeArgs := []string{"logs", "--tail", "200"}
		if len(args) > 1 {
			composeArgs = append(composeArgs, args[1:]...)
		}
		if err := runCompose(append([]string{"-p", "solas"}, composeArgs...)...); err != nil {
			fmt.Fprintf(os.Stderr, "failed to stream stack logs: %v\n", err)
			return true, 1
		}
		return true, 0
	default:
		return false, 0
	}
}

func ensureOllamaOnHost() error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(ollamaHealthURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status from %s: %s", ollamaHealthURL, resp.Status)
	}
	return nil
}

func runCompose(args ...string) error {
	composeCmd, err := detectComposeCommand()
	if err != nil {
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	composeFile := filepath.Join(repoRoot, "solas-stack", "docker-compose.yml")
	cmdArgs := append(composeCmd[1:], "-f", composeFile)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(composeCmd[0], cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func detectComposeCommand() ([]string, error) {
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		return []string{"docker", "compose"}, nil
	}
	if err := exec.Command("docker-compose", "version").Run(); err == nil {
		return []string{"docker-compose"}, nil
	}
	return nil, errors.New("could not find Docker Compose (`docker compose` or `docker-compose`) in PATH")
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(cwd, "go.mod")
		stackPath := filepath.Join(cwd, "solas-stack")
		if fileExists(goModPath) && dirExists(stackPath) {
			return cwd, nil
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	return "", errors.New("could not locate repository root containing go.mod and solas-stack")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
