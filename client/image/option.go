package image

type Options struct {
	chunkSize string
	threshold string
}

type Option func(*Options)

func WithChunSize(sz string) Option {
	return func(opts *Options) {
		opts.chunkSize = sz
	}
}

// when image is bigger than threshold, we use chunk upload and download
func WithChunkThreshold(sz string) Option {
	return func(opts *Options) {
		opts.threshold = sz
	}
}

type PullPolicy string

const (
	PullPolicyAlways       = "Always"
	PullPolicyIfNotPresent = "IfNotPresent"
	PullPolicyNever        = "Never"
)
