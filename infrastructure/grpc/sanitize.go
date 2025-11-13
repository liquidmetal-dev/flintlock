package grpc

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/api/types"
)

func sanitizeMicroVMImageReferences(logger *logrus.Entry, spec *types.MicroVMSpec) {
	if spec == nil {
		return
	}

	if spec.Kernel != nil {
		sanitizedValue, changed := cleanContainerImageReference(spec.Kernel.Image)
		if changed {
			logSanitizedField(logger, "kernel.image", spec.Kernel.Image, sanitizedValue)
			spec.Kernel.Image = sanitizedValue
		}
	}

	if spec.Initrd != nil {
		sanitizedValue, changed := cleanContainerImageReference(spec.Initrd.Image)
		if changed {
			logSanitizedField(logger, "initrd.image", spec.Initrd.Image, sanitizedValue)
			spec.Initrd.Image = sanitizedValue
		}
	}

	sanitizeVolumeImage(logger, "rootVolume", spec.RootVolume)

	for index, volume := range spec.AdditionalVolumes {
		fieldName := fmt.Sprintf("additionalVolumes[%d]", index)
		sanitizeVolumeImage(logger, fieldName, volume)
	}
}

func sanitizeVolumeImage(logger *logrus.Entry, fieldName string, volume *types.Volume) {
	if volume == nil || volume.Source == nil || volume.Source.ContainerSource == nil {
		return
	}

	originalValue := *volume.Source.ContainerSource
	sanitizedValue, changed := cleanContainerImageReference(originalValue)
	if !changed {
		return
	}

	logSanitizedField(logger, fieldName+".containerSource", originalValue, sanitizedValue)
	volume.Source.ContainerSource = &sanitizedValue
}

func cleanContainerImageReference(raw string) (string, bool) {
	if raw == "" {
		return raw, false
	}

	trimmed := strings.TrimSpace(raw)

	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}

		if unicode.IsControl(r) {
			return -1
		}

		return r
	}, trimmed)

	if cleaned == raw {
		return cleaned, false
	}

	return cleaned, true
}

func logSanitizedField(logger *logrus.Entry, field string, originalValue, sanitizedValue string) {
	logger.WithFields(logrus.Fields{
		"field":          field,
		"originalImage":  originalValue,
		"sanitizedImage": sanitizedValue,
	}).Warn("sanitized container image reference before Flintlock processing")
}
