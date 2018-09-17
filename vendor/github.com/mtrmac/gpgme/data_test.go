package gpgme

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestNewData(t *testing.T) {
	dh, err := NewData()
	checkError(t, err)
	for i := 0; i < 5; i++ {
		_, err := dh.Write([]byte(testData))
		checkError(t, err)
	}
	_, err = dh.Seek(0, SeekSet)
	checkError(t, err)

	var buf bytes.Buffer
	_, err = io.Copy(&buf, dh)
	checkError(t, err)
	expected := bytes.Repeat([]byte(testData), 5)
	diff(t, buf.Bytes(), expected)

	dh.Close()
}

func TestNewDataBytes(t *testing.T) {
	// Test ordinary data, and empty slices
	for _, content := range [][]byte{[]byte("content"), []byte{}} {
		dh, err := NewDataBytes(content)
		checkError(t, err)

		_, err = dh.Seek(0, SeekSet)
		checkError(t, err)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, dh)
		checkError(t, err)
		diff(t, buf.Bytes(), content)
	}
}

func TestDataNewDataFile(t *testing.T) {
	f, err := ioutil.TempFile("", "gpgme")
	checkError(t, err)
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	dh, err := NewDataFile(f)
	checkError(t, err)
	defer dh.Close()
	for i := 0; i < 5; i++ {
		_, err := dh.Write([]byte(testData))
		checkError(t, err)
	}
	_, err = dh.Seek(0, SeekSet)
	checkError(t, err)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, dh)
	checkError(t, err)
	expected := bytes.Repeat([]byte(testData), 5)
	diff(t, buf.Bytes(), expected)
}

func TestDataNewDataReader(t *testing.T) {
	r := bytes.NewReader([]byte(testData))
	dh, err := NewDataReader(r)
	checkError(t, err)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, dh)
	checkError(t, err)
	diff(t, buf.Bytes(), []byte(testData))

	dh.Close()
}

func TestDataNewDataWriter(t *testing.T) {
	var buf bytes.Buffer
	dh, err := NewDataWriter(&buf)
	checkError(t, err)
	for i := 0; i < 5; i++ {
		_, err := dh.Write([]byte(testData))
		checkError(t, err)
	}
	expected := bytes.Repeat([]byte(testData), 5)
	diff(t, buf.Bytes(), expected)

	dh.Close()
}
