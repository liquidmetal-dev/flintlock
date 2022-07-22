package godisk

import (
	"context"
	"encoding/base64"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
)

func TestDiskCreation(t *testing.T) {
	g.RegisterTestingT(t)

	imagePath := "test.img"
	filePath := "/file.txt"
	fileContent := base64.StdEncoding.EncodeToString([]byte("testing"))
	ctx := context.TODO()
	expectedSize := int64(8000000)

	fs := afero.NewOsFs()
	svc := New(fs)

	defer testCleanupImage(imagePath, fs)

	input := ports.DiskCreateInput{
		Path:       imagePath,
		Size:       "8Mb",
		VolumeName: "data",
		Type:       ports.DiskTypeFat32,
		Files: []ports.DiskFile{
			{
				Path:          filePath,
				ContentBase64: fileContent,
			},
		},
	}

	err := svc.Create(ctx, input)
	g.Expect(err).NotTo(g.HaveOccurred())

	exists, err := afero.Exists(fs, imagePath)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(exists).To(g.BeTrue())

	info, err := fs.Stat(imagePath)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(info.Size()).To(g.Equal(expectedSize))

	//TODO: in the future we could consider inspecting the created image.
}

func TestInvalidSize(t *testing.T) {
	g.RegisterTestingT(t)

	imagePath := "test.img"
	filePath := "/file.txt"
	fileContent := base64.StdEncoding.EncodeToString([]byte("testing"))
	ctx := context.TODO()

	fs := afero.NewOsFs()
	svc := New(fs)

	defer testCleanupImage(imagePath, fs)

	input := ports.DiskCreateInput{
		Path:       imagePath,
		Size:       "8xx",
		VolumeName: "data",
		Type:       ports.DiskTypeFat32,
		Files: []ports.DiskFile{
			{
				Path:          filePath,
				ContentBase64: fileContent,
			},
		},
	}

	err := svc.Create(ctx, input)
	g.Expect(err).To(g.HaveOccurred())

}

func testCleanupImage(imagePath string, fs afero.Fs) {
	exists, err := afero.Exists(fs, imagePath)
	if err != nil {
		return
	}
	if !exists {
		return
	}
	fs.Remove(imagePath)
}
