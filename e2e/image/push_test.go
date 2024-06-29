package image

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2etypes "github.com/projecteru2/vmihub/e2e/types"
	utils "github.com/projecteru2/vmihub/internal/utils"
	"github.com/projecteru2/vmihub/pkg/types"
)

var _ = Describe("Push image", func() {
	Describe("With single file", func() {
		It("Successfully", func() {
			imageAPI, baseDir := newImageAPI(6*1024*1024, 10*1024*1024)
			defer os.RemoveAll(baseDir)

			cfg, err := e2etypes.LoadConfig(configFile)
			Expect(err).To(BeNil())

			testCases := []struct {
				user string
				name string
				tag  string
				size int64
			}{
				{
					user: cfg.Username,
					name: "push-test-image",
					tag:  "test-tag",
					size: 1024 * 1024,
				},
				{
					user: cfg.Username,
					name: "push-test-image2",
					tag:  "latest",
					size: 1024 * 1024,
				},
				// upload with chunk
				{
					user: cfg.Username,
					name: "push-test-image-chunk",
					tag:  "test-tag",
					size: 13 * 1024 * 1024,
				},
				{
					user: cfg.Username,
					name: "push-test-image-chunk2",
					tag:  "latest",
					size: 13 * 1024 * 1024,
				},
			}
			for _, tc := range testCases {
				ctx := context.TODO()
				fullname := fmt.Sprintf("%s/%s:%s", tc.user, tc.name, tc.tag)
				testImg, err := imageAPI.NewImage(fullname)
				Expect(err).To(BeNil())
				testImg.Format = "qcow2"
				testImg.OS = types.OSInfo{
					Type:    "linux",
					Distrib: "ubuntu",
					Version: "20.04",
					Arch:    "amd64",
				}

				fname, err := prepareFile(tc.size, 'a')
				Expect(err).To(BeNil())
				defer os.Remove(fname)

				err = testImg.CopyFrom(fname)
				Expect(err).To(BeNil())

				err = imageAPI.Push(ctx, testImg, false)
				Expect(err).To(BeNil())
				defer func() {
					err = imageAPI.RemoveImage(ctx, testImg)
					Expect(err).To(BeNil())
				}()
				info, err := imageAPI.GetInfo(ctx, fullname)
				Expect(err).To(BeNil())
				Expect(info.Digest).To(Equal(testImg.Digest))
				Expect(info.Username).To(Equal(tc.user))
				Expect(info.Name).To(Equal(tc.name))
				Expect(info.Tag).To(Equal(utils.NormalizeTag(tc.tag, info.Digest)))
			}
		})
		It("failed", func() {
		})
	})
})
