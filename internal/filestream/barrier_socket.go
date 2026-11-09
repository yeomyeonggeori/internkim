package filestream

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	barrierRequest     = "wait"
	barrierKept        = "kept"
	barrierUnjournaled = "unjournaled"
	barrierWaitLimit   = 30 * time.Second
)

func (barrier *Barrier) Serve(ctx context.Context, socketPath string, mode os.FileMode) error {
	if errorValue := os.Remove(socketPath); errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		return fmt.Errorf("opening the file journal's barrier at %s: %w", socketPath, errorValue)
	}
	if errorValue := os.Chmod(socketPath, mode); errorValue != nil {
		listener.Close()
		return errorValue
	}
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	for {
		connection, errorValue := listener.Accept()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errorValue != nil {
			return fmt.Errorf("answering the file journal's barrier: %w", errorValue)
		}
		go barrier.answer(ctx, connection)
	}
}

func (barrier *Barrier) answer(ctx context.Context, connection net.Conn) {
	defer connection.Close()
	request, errorValue := bufio.NewReader(connection).ReadString('\n')
	if errorValue != nil || strings.TrimSpace(request) != barrierRequest {
		fmt.Fprintf(connection, "refused: send %q and a newline\n", barrierRequest)
		return
	}
	if barrier.isUnjournaled {
		fmt.Fprintln(connection, barrierUnjournaled)
		return
	}
	waiting, cancel := context.WithTimeout(ctx, barrierWaitLimit)
	defer cancel()
	if errorValue := barrier.Wait(waiting); errorValue != nil {
		log.Printf("a writer asked for the company's files to be kept and they were not: %v", errorValue)
		fmt.Fprintf(connection, "failed: %v\n", errorValue)
		return
	}
	fmt.Fprintln(connection, barrierKept)
}
