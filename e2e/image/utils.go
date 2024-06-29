package image

import (
	"bytes"
	"os"
)

func generateFile(filename string, sizeInBytes int64, b byte) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write bytes to the file until it reaches the specified size
	var bytesWritten int64
	for bytesWritten < sizeInBytes {
		bytesToWrite := sizeInBytes - bytesWritten
		if bytesToWrite > 1024 {
			bytesToWrite = 1024 // Write at most 1 KB at a time
		}
		buf := bytes.Repeat([]byte{b}, int(bytesToWrite))
		n, err := file.Write(buf)
		if err != nil {
			return err
		}
		bytesWritten += int64(n)
	}

	return nil
}

func generateTempFilename() string {
	// Generate a temporary filename
	tempFile, err := os.CreateTemp("/tmp", "example")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tempFile.Name()) // Remove the temporary file immediately
	return tempFile.Name()
}

func prepareFile(sz int64, b byte) (string, error) {
	fname := generateTempFilename()
	err := generateFile(fname, sz, b)
	return fname, err
}
