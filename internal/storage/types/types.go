package types

import (
	"encoding/json"
	"io"
)

type ChunkInfo struct {
	Idx       int           `json:"idx"`
	Size      int64         `json:"size"`
	ChunkSize int64         `json:"chunk_size"`
	Digest    string        `json:"digest"`
	In        io.ReadSeeker `json:"-"`
	Raw       any           `json:"raw"`
}

func (ci *ChunkInfo) MarshalBinary() (data []byte, err error) {
	return json.Marshal(ci)
}

func (ci *ChunkInfo) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, ci)
}
