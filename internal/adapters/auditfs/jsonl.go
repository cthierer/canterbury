package auditfs

import (
	"encoding/json"
	"fmt"
	"io"
)

func appendJSONL(writer io.Writer, event eventData) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event data: %w", err)
	}

	data = append(data, '\n')

	written, err := writer.Write(data)
	if err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}

	if written != len(data) {
		return fmt.Errorf("write audit event: %w", io.ErrShortWrite)
	}

	return nil
}
