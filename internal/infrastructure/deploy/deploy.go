package deploy

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// Target represents a remote machine for worker deployment.
type Target struct {
	Host     string
	Port     int
	User     string
	Password string // optional: use password auth
	KeyFile  string // optional: use SSH key auth
}

// WorkerSpec defines the worker to deploy.
type WorkerSpec struct {
	ServerURL   string
	WorkerToken string
	WorkerName  string
	WorkerType  string
	LogToDB     bool
}

// DeployWorker deploys a worker binary to the remote target and starts it.
// The localBinary is the path to the compiled ADP binary that supports the "worker run" subcommand.
func DeployWorker(target Target, spec WorkerSpec, localBinary string) error {
	client, err := connect(target)
	if err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}
	defer func() { _ = client.Close() }()

	remotePath := "/tmp/adp-worker"
	tokenPath := "/tmp/adp-worker.token"

	// 1. Upload binary.
	log.Printf("deploy: uploading binary to %s@%s:%s", target.User, target.Host, remotePath)
	if err := uploadFile(client, localBinary, remotePath); err != nil {
		return fmt.Errorf("upload binary: %w", err)
	}
	if err := uploadBytes(client, []byte(spec.WorkerToken+"\n"), tokenPath); err != nil {
		return fmt.Errorf("upload worker token: %w", err)
	}
	if err := runCmd(client, fmt.Sprintf("chmod 600 %s", tokenPath)); err != nil {
		return fmt.Errorf("protect worker token file: %w", err)
	}

	// 2. Make executable.
	if err := runCmd(client, fmt.Sprintf("chmod +x %s", remotePath)); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// 3. Kill any existing worker process.
	if err := runCmd(client, "pkill -f 'adp-worker' 2>/dev/null; pkill -f '/tmp/adp-worker worker run' 2>/dev/null; true"); err != nil {
		return fmt.Errorf("stop existing worker: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 4. Start worker in background.
	logToDBFlag := ""
	if spec.LogToDB {
		logToDBFlag = " --log-to-db"
	}
	startCmd := fmt.Sprintf(
		"nohup %s worker run --server-url=%s --worker-token-file=%s --worker-name=%s --worker-type=%s%s > /tmp/adp-worker.log 2>&1 &",
		remotePath, spec.ServerURL, tokenPath, spec.WorkerName, spec.WorkerType, logToDBFlag,
	)
	log.Printf("deploy: starting worker %s on %s", spec.WorkerName, target.Host)
	if err := runCmd(client, startCmd); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	log.Printf("deploy: worker %s deployed to %s@%s", spec.WorkerName, target.User, target.Host)
	return nil
}

// connect establishes an SSH connection to the target.
func connect(target Target) (*ssh.Client, error) {
	if target.Port == 0 {
		target.Port = 22
	}

	var authMethods []ssh.AuthMethod
	if target.Password != "" {
		authMethods = append(authMethods, ssh.Password(target.Password))
	}
	if target.KeyFile != "" {
		key, err := os.ReadFile(target.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no auth method configured (password or key_file required)")
	}

	config := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // for internal use
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)
	return ssh.Dial("tcp", addr, config)
}

// uploadFile copies a local file to the remote host via stdin piping.
func uploadFile(client *ssh.Client, localPath, remotePath string) error {
	file, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	return uploadBytes(client, file, remotePath)
}

func uploadBytes(client *ssh.Client, file []byte, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	// Write file content to remote via a cat redirect.
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer func() { _ = stdin.Close() }()
		_, _ = stdin.Write(file)
	}()

	// Use base64 to avoid binary corruption over SSH.
	// Simplified: use cat + redirect for the binary.
	remoteDir := filepath.Dir(remotePath)
	remoteName := filepath.Base(remotePath)
	cmd := fmt.Sprintf("mkdir -p %s && cat > %s/%s", remoteDir, remoteDir, remoteName)
	return session.Run(cmd)
}

// runCmd executes a command on the remote host.
func runCmd(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}
