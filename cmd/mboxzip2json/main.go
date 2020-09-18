package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-mbox"
)

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mainErr() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("missing file name")
	}

	emails, err := extractEmailsFromZip(os.Args[1])
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(emails)
}

type email struct {
	From      string
	To        string
	CC        string
	Subject   string
	Body      string
	Timestamp time.Time
}

func extractEmailsFromZip(path string) ([]email, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %v", err)
	}

	z, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("creating zip reader: %v", err)
	}

	emails := make([]email, 0)

	if err := descendZip(path, z, func(zf *zip.File) error {
		fileName := zf.FileInfo().Name()

		switch true {
		case strings.HasSuffix(fileName, ".mbox"):
		case strings.HasSuffix(fileName, ".mbox.txt"):
		default:
			return nil
		}

		r, err := zf.Open()
		if err != nil {
			return fmt.Errorf("open archived file: %v", err)
		}
		defer r.Close()

		ms := mbox.NewScanner(r)
		for ms.Next() {
			m := ms.Message()

			bodyBytes, err := ioutil.ReadAll(m.Body)
			if err != nil {
				return fmt.Errorf("read message body: %v", err)
			}

			xh := func(k string) string {
				return strings.Join(m.Header[k], ", ")
			}

			ts, err := newTimeParser(
				"Mon, 2 Jan 2006 15:04:05 -0700",
				"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
				"Mon, 2 Jan 2006 15:04:05 MST",
			).Parse(xh("Date"))
			if err != nil {
				return fmt.Errorf("reading Date header: %v", err)
			}

			emails = append(emails, email{
				From:      xh("From"),
				To:        xh("To"),
				CC:        xh("Cc"),
				Subject:   xh("Subject"),
				Body:      string(bodyBytes),
				Timestamp: ts,
			})
		}

		if err := ms.Err(); err != nil {
			return fmt.Errorf("scanning mbox messages: %v", err)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("reading zip file: %v", err)
	}

	return emails, nil
}

func descendZip(pathPrefix string, z *zip.Reader, fn func(*zip.File) error) error {
	for _, f := range z.File {
		fi := f.FileInfo()
		if fi.IsDir() {
			continue
		}

		r, err := f.Open()
		if err != nil {
			return fmt.Errorf("open archive file: %v", err)
		}
		defer r.Close()

		fullPath := filepath.Join(pathPrefix, fi.Name())

		switch filepath.Ext(fi.Name()) {
		case ".zip":
			zr, err := newZipFromReader(r, fi.Size())
			if err != nil {
				return fmt.Errorf("newZipFromReader: %v", err)
			}
			if err := descendZip(fullPath, zr, fn); err != nil {
				return err
			}
		default:
			if err := fn(f); err != nil {
				return fmt.Errorf("processing file %s: %v", fullPath, err)
			}
		}
	}

	return nil
}

func newZipFromReader(file io.ReadCloser, size int64) (*zip.Reader, error) {
	in := file.(io.Reader)

	if _, ok := in.(io.ReaderAt); ok != true {
		buffer, err := ioutil.ReadAll(in)

		if err != nil {
			return nil, fmt.Errorf("read all into buffer: %v", err)
		}

		in = bytes.NewReader(buffer)
		size = int64(len(buffer))
	}

	reader, err := zip.NewReader(in.(io.ReaderAt), size)

	if err != nil {
		return nil, fmt.Errorf("creating new zip reader: %v", err)
	}

	return reader, nil
}

type timeParser struct {
	formats []string
}

func newTimeParser(formats ...string) *timeParser {
	return &timeParser{
		formats: formats,
	}
}

func (p *timeParser) Parse(s string) (time.Time, error) {
	var lastErr error
	for _, f := range p.formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}
