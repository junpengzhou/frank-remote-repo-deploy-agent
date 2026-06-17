package register

import (
	"context"
	"fmt"
	"io"
	"os"
)

type DryRunClient struct {
	Writer io.Writer
}

func NewDryRunClient(writer io.Writer) *DryRunClient {
	if writer == nil {
		writer = os.Stdout
	}
	return &DryRunClient{Writer: writer}
}

func (c *DryRunClient) Run(_ context.Context, command string) error {
	_, _ = fmt.Fprintf(c.Writer, "[register cmd] %s\n", command)
	return nil
}

func (c *DryRunClient) MkdirAll(_ context.Context, path string) error {
	_, _ = fmt.Fprintf(c.Writer, "[register mkdir] %s\n", path)
	return nil
}

func (c *DryRunClient) UploadFile(_ context.Context, localPath, remotePath string, mode os.FileMode) error {
	_, _ = fmt.Fprintf(c.Writer, "[register upload] %s -> %s (%#o)\n", localPath, remotePath, mode)
	return nil
}

func (c *DryRunClient) Close() error {
	return nil
}
