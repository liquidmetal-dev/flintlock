package shared

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/spf13/afero"
)

func PIDReadFromFile(pidFile string, fs afero.Fs) (int, error) {
	file, err := fs.Open(pidFile)
	if err != nil {
		return -1, fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return -1, fmt.Errorf("reading pid file %s: %w", pidFile, err)
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return -1, fmt.Errorf("converting data to int: %w", err)
	}

	return pid, nil
}

func PIDWriteToFile(pid int, pidFile string, fs afero.Fs) error {
	file, err := fs.OpenFile(pidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", pid)
	if err != nil {
		return fmt.Errorf("writing pid %d to file %s: %w", pid, pidFile, err)
	}

	return nil
}
