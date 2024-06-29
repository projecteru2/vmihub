package image

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libimage "github.com/projecteru2/vmihub/client/image"
	e2etypes "github.com/projecteru2/vmihub/e2e/types"
	utils "github.com/projecteru2/vmihub/internal/utils"
)

var _ = Describe("Pull image", func() {
	Describe("With single file", func() {
		It("Successfully", func() {
			imageAPI, baseDir := newImageAPI(0, 0)
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
					name: "pull-test-image",
					tag:  "test-tag",
					size: 1024 * 1024,
				},
				{
					user: cfg.Username,
					name: "pull-test-image2",
					tag:  "latest",
					size: 1024 * 1024,
				},
			}
			for _, tc := range testCases {
				ctx := context.TODO()
				testImg := createImage(ctx, imageAPI, tc.user, tc.name, tc.tag, 'a', tc.size)
				defer func() {
					err = imageAPI.RemoveImage(ctx, testImg)
					Expect(err).To(BeNil())
				}()
				err = imageAPI.RemoveLocalImage(ctx, testImg)
				Expect(err).To(BeNil())
				cached, err := testImg.Cached()
				Expect(err).To(BeNil())
				Expect(cached).To(BeFalse())

				newImg, err := imageAPI.Pull(ctx, testImg.Fullname(), libimage.PullPolicyAlways)
				Expect(err).To(BeNil())
				Expect(newImg.Username).To(Equal(testImg.Username))
				Expect(newImg.Name).To(Equal(testImg.Name))
				Expect(newImg.Tag).To(Equal(utils.NormalizeTag(tc.tag, newImg.Digest)))

				cached1, err := testImg.Cached()
				Expect(err).To(BeNil())

				cached2, err := newImg.Cached()
				Expect(err).To(BeNil())
				if tc.tag == "" || tc.tag == "latest" {
					Expect(cached1).To(BeFalse())
					Expect(cached2).To(BeTrue())
				} else {
					Expect(cached1).To(BeTrue())
					Expect(cached2).To(BeTrue())
				}
			}
		})
		It("latest successfully", func() {
			imageAPI, baseDir := newImageAPI(0, 0)
			defer os.RemoveAll(baseDir)

			cfg, err := e2etypes.LoadConfig(configFile)
			Expect(err).To(BeNil())
			ctx := context.TODO()
			name := "pull-test-latest-tag"
			for idx := 0; idx < 3; idx++ {
				testImg := createImage(ctx, imageAPI, cfg.Username, name, "latest", byte(idx+65), 1024*1024)
				defer func() {
					err = imageAPI.RemoveImage(ctx, testImg)
					Expect(err).To(BeNil())
				}()
				digest := testImg.Digest
				err = imageAPI.RemoveLocalImage(ctx, testImg)
				Expect(err).To(BeNil())
				cached, err := testImg.Cached()
				Expect(err).To(BeNil())
				Expect(cached).To(BeFalse())

				newImg, err := imageAPI.Pull(ctx, testImg.Fullname(), libimage.PullPolicyAlways)
				Expect(err).To(BeNil())
				Expect(newImg.Username).To(Equal(testImg.Username))
				Expect(newImg.Name).To(Equal(testImg.Name))
				Expect(newImg.Tag).To(Equal(utils.NormalizeTag("latest", newImg.Digest)))
				Expect(newImg.Digest).To(Equal(digest))

				cached, err = testImg.Cached()
				Expect(err).To(BeNil())
				Expect(cached).To(BeFalse())

				cached, err = newImg.Cached()
				Expect(err).To(BeNil())
				Expect(cached).To(BeTrue())
			}
		})
		It("failed", func() {
		})
	})
	Describe("With chunk", func() {
		It("Successfully", func() {
			imageAPI, baseDir := newImageAPI(6*1024*1024, 10*1024*1024)
			_ = baseDir
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
					name: "pull-test-image-chunk",
					tag:  "test-tag2",
					size: 13 * 1024 * 1024,
				},
				{
					user: cfg.Username,
					name: "pull-test-image-chunk2",
					tag:  "latest",
					size: 13 * 1024 * 1024,
				},
			}
			for _, tc := range testCases {
				ctx := context.TODO()
				testImg := createImage(ctx, imageAPI, tc.user, tc.name, tc.tag, 'a', tc.size)
				defer func() {
					err = imageAPI.RemoveImage(ctx, testImg)
					Expect(err).To(BeNil())
				}()
				err = imageAPI.RemoveLocalImage(ctx, testImg)
				Expect(err).To(BeNil())
				cached, err := testImg.Cached()
				Expect(err).To(BeNil())
				Expect(cached).To(BeFalse())

				newImg, err := imageAPI.Pull(ctx, testImg.Fullname(), libimage.PullPolicyAlways)
				Expect(err).To(BeNil())
				Expect(newImg.Username).To(Equal(testImg.Username))
				Expect(newImg.Name).To(Equal(testImg.Name))
				Expect(newImg.Tag).To(Equal(utils.NormalizeTag(tc.tag, newImg.Digest)))

				cached1, err := testImg.Cached()
				Expect(err).To(BeNil())

				cached2, err := newImg.Cached()
				Expect(err).To(BeNil())

				if tc.tag == "" || tc.tag == "latest" {
					Expect(cached1).To(BeFalse())
					Expect(cached2).To(BeTrue())
				} else {
					Expect(cached1).To(BeTrue())
					Expect(cached2).To(BeTrue())
				}
			}
		})
		It("failed", func() {
		})
	})
})
