package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type localTestEmbeddingService struct {
	command  *exec.Cmd
	endpoint string
	logFile  *os.File
}

func startLocalTestEmbeddingService(ctx context.Context, repositoryRootPath string) (*localTestEmbeddingService, error) {
	serverPath, modelPath, errorValue := prepareLocalTestEmbedding(ctx, repositoryRootPath)
	if errorValue != nil {
		return nil, errorValue
	}
	port, errorValue := availableLocalPort()
	if errorValue != nil {
		return nil, errorValue
	}
	logFile, errorValue := localTestEmbeddingLogFile(repositoryRootPath)
	if errorValue != nil {
		return nil, errorValue
	}
	command := exec.Command(serverPath,
		"-m", modelPath,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--device", "none",
		"-ngl", "0",
		"--fit", "off",
		"--no-op-offload",
		"--embeddings",
		"--pooling", "cls",
		"--batch-size", "2048",
		"--ubatch-size", "2048",
	)
	command.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+filepath.Dir(serverPath), "LD_LIBRARY_PATH="+filepath.Dir(serverPath))
	command.Stdout = logFile
	command.Stderr = logFile
	if errorValue := command.Start(); errorValue != nil {
		logFile.Close()
		return nil, errorValue
	}
	service := &localTestEmbeddingService{
		command:  command,
		endpoint: "http://127.0.0.1:" + strconv.Itoa(port) + "/v1/embeddings",
		logFile:  logFile,
	}
	if errorValue := service.waitUntilReady(ctx); errorValue != nil {
		service.stop()
		return nil, errorValue
	}
	return service, nil
}

func prepareLocalTestEmbedding(ctx context.Context, repositoryRootPath string) (string, string, error) {
	command := exec.CommandContext(ctx, filepath.Join(repositoryRootPath, "tools", "prepare-local-test-embedding"))
	command.Dir = repositoryRootPath
	command.Stderr = os.Stderr
	output, errorValue := command.Output()
	if errorValue != nil {
		return "", "", errorValue
	}
	paths := trimmedNonEmptyValues(strings.Split(string(output), "\n"))
	if len(paths) != 2 {
		return "", "", errors.New("local embedding preparation did not return server and model paths")
	}
	return paths[0], paths[1], nil
}

func availableLocalPort() (int, error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, errorValue
	}
	defer listener.Close()
	address, isTCPAddress := listener.Addr().(*net.TCPAddr)
	if !isTCPAddress {
		return 0, errors.New("local embedding listener did not allocate a TCP port")
	}
	return address.Port, nil
}

func localTestEmbeddingLogFile(repositoryRootPath string) (*os.File, error) {
	artifactDirectoryPath := filepath.Join(repositoryRootPath, ".artifacts", "expensive")
	if errorValue := os.MkdirAll(artifactDirectoryPath, 0o755); errorValue != nil {
		return nil, errorValue
	}
	return os.Create(filepath.Join(artifactDirectoryPath, "local-embedding.log"))
}

func (service *localTestEmbeddingService) waitUntilReady(ctx context.Context) error {
	startupDeadline := time.NewTimer(2 * time.Minute)
	defer startupDeadline.Stop()
	for {
		if errorValue := service.verifyEmbedding(ctx); errorValue == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-startupDeadline.C:
			return errors.New("local BGE-M3 embedding server did not become ready")
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (service *localTestEmbeddingService) verifyEmbedding(ctx context.Context) error {
	requestDocument, errorValue := json.Marshal(map[string]string{"model": "baai/bge-m3", "input": "로컬 임베딩 확인"})
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := http.DefaultClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("local embedding health returned %s", response.Status)
	}
	var responseDocument struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&responseDocument); errorValue != nil {
		return errorValue
	}
	if len(responseDocument.Data) != 1 || len(responseDocument.Data[0].Embedding) != 1024 {
		return errors.New("local BGE-M3 embedding response must contain one 1024-dimensional vector")
	}
	return nil
}

func (service *localTestEmbeddingService) stop() {
	if service == nil {
		return
	}
	if service.command != nil && service.command.Process != nil {
		_ = service.command.Process.Signal(os.Interrupt)
		waitDone := make(chan struct{})
		go func() {
			_ = service.command.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(5 * time.Second):
			_ = service.command.Process.Kill()
			<-waitDone
		}
	}
	if service.logFile != nil {
		_ = service.logFile.Close()
	}
}
