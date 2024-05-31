package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type PublicKeyRequest struct {
	Title string `json:"title"`
	Key   string `json:"key"`
}

func main() {
	fmt.Print("Enter your email: ")
	var email string
	fmt.Scanln(&email)

	fmt.Print("Enter your passphrase: ")
	bytePassphrase, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading passphrase: %v\n", err)
		return
	}
	passphrase := string(bytePassphrase)
	fmt.Println()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key pair: %v\n", err)
		return
	}

	block := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: privateKey.Seed(),
	}
	encryptedBlock, err := x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte(passphrase), x509.PEMCipherAES256)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encrypting private key: %v\n", err)
		return
	}
	privateKeyBytes := pem.EncodeToMemory(encryptedBlock)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		return
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	err = os.MkdirAll(sshDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .ssh directory: %v\n", err)
		return
	}

	privateKeyPath := filepath.Join(sshDir, "id_ed25519")
	err = os.WriteFile(privateKeyPath, privateKeyBytes, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving private key: %v\n", err)
		return
	}

	publicKeyBytes, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling public key: %v\n", err)
		return
	}

	publicKeyPath := filepath.Join(sshDir, "id_ed25519.pub")
	err = os.WriteFile(publicKeyPath, ssh.MarshalAuthorizedKey(publicKeyBytes), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving public key: %v\n", err)
		return
	}

	err = startSSHAgentAndSetEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting ssh-agent: %v\n", err)
		return
	}

	err = addKeyToSSHAgent(privateKeyPath, passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding key to ssh-agent: %v\n", err)
		return
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		fmt.Fprintf(os.Stderr, "Error: GITHUB_TOKEN environment variable not set\n")
		return
	}

	err = addKeyToGitHub(ssh.MarshalAuthorizedKey(publicKeyBytes), githubToken, publicKeyPath, email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding key to GitHub: %v\n", err)
		return
	}

	fmt.Println("Key added successfully")
}

func startSSHAgentAndSetEnv() error {
	cmd := exec.Command("ssh-agent", "-s")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start ssh-agent: %v", err)
	}

	lines := bytes.Split(out.Bytes(), []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("export")) {
			parts := bytes.SplitN(line, []byte(" "), 2)
			if len(parts) == 2 {
				envParts := bytes.SplitN(parts[1], []byte("="), 2)
				if len(envParts) == 2 {
					os.Setenv(string(envParts[0]), string(envParts[1]))
				}
			}
		}
	}
	return nil
}

func addKeyToSSHAgent(privateKeyPath, passphrase string) error {
	cmd := exec.Command("ssh-add", privateKeyPath)
	cmd.Stdin = bytes.NewReader([]byte(passphrase + "\n"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to add key to ssh-agent: %v", err)
	}
	return nil
}

func addKeyToGitHub(pubKey []byte, githubToken string, publicKeyPath string, email string) error {
	reqBody := PublicKeyRequest{
		Title: "SSH Key for " + email,
		Key:   string(pubKey),
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("Error when trying to marshal request body: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.github.com/user/keys", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return fmt.Errorf("Error when trying to create new request: %v", err)
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Error when trying to send request to GitHub: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		errorMessage := `Error when trying to add key to GitHub: %s
		You can upload the key manually by going to GitHub > Settings > SSH and GPG keys > New SSH key. 
		Paste your public key into the 'Key' field. 
		Your public key is located at: %s`
		return fmt.Errorf(errorMessage, body, publicKeyPath)
	}

	return nil
}
