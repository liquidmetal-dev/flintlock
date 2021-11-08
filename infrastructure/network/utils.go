package network

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/weaveworks/flintlock/core/models"
)

const (
	ifaceLength   = 7
	retryGenerate = 5
	prefix        = "fl"
	tapPrefix     = "tap"
	// It's vtap only to save space.
	macvtapPrefix = "vtap"
)

func generateRandomName(prefix string) (string, error) {
	id := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}

	return prefix + hex.EncodeToString(id)[:ifaceLength], nil
}

func NewIfaceName(ifaceType models.IfaceType) (string, error) {
	devPrefix := ""

	switch ifaceType {
	case models.IfaceTypeTap:
		devPrefix = fmt.Sprintf("%s%s", prefix, tapPrefix)
	case models.IfaceTypeMacvtap:
		devPrefix = fmt.Sprintf("%s%s", prefix, macvtapPrefix)
	default:
		return "", interfaceErrorf("unsupported interface type: %s", ifaceType)
	}

	for i := 0; i < retryGenerate; i++ {
		name, err := generateRandomName(devPrefix)
		if err != nil {
			continue
		}

		_, err = netlink.LinkByName(name)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return name, nil
			}

			return "", interfaceErrorf("unable to get link by name: %s", err.Error())
		}
	}

	return "", interfaceErrorf("could not generate interface name")
}
