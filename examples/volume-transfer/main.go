package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"

	libvirt "github.com/Ns2Kracy/libvirt-go"
)

type options struct {
	uri         string
	direction   string
	pool        string
	volume      string
	file        string
	offset      uint64
	length      uint64
	flags       uint
	streamFlags uint
}

func main() {
	var opts options
	flag.StringVar(&opts.uri, "uri", "qemu:///session", "libvirt connection URI")
	flag.StringVar(&opts.direction, "direction", "download", "upload or download")
	flag.StringVar(&opts.pool, "pool", "", "storage pool name")
	flag.StringVar(&opts.volume, "volume", "", "storage volume name")
	flag.StringVar(&opts.file, "file", "", "local input or output file")
	flag.Uint64Var(&opts.offset, "offset", 0, "byte offset within the volume")
	flag.Uint64Var(&opts.length, "length", 0, "maximum bytes to transfer; zero means all remaining data")
	flag.UintVar(&opts.flags, "flags", 0, "volume transfer flags as an unsigned integer")
	flag.UintVar(&opts.streamFlags, "stream-flags", 0, "stream flags as an unsigned integer; use zero for blocking I/O")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

func run(opts options) (err error) {
	opts.direction = strings.ToLower(opts.direction)
	if opts.direction != "upload" && opts.direction != "download" {
		return fmt.Errorf("unknown direction %q", opts.direction)
	}
	if opts.pool == "" || opts.volume == "" || opts.file == "" {
		return errors.New("-pool, -volume, and -file are required")
	}
	if opts.flags > math.MaxUint32 || opts.streamFlags > math.MaxUint32 {
		return errors.New("flags values must fit in uint32")
	}
	if opts.length > math.MaxInt64 {
		return errors.New("-length must fit in int64 for io.CopyN")
	}

	var conn *libvirt.Connect
	if opts.direction == "download" {
		conn, err = libvirt.NewConnectReadOnly(opts.uri)
	} else {
		conn, err = libvirt.NewConnect(opts.uri)
	}
	if err != nil {
		return fmt.Errorf("open %q: %w", opts.uri, err)
	}
	defer func() {
		_, closeErr := conn.Close()
		err = errors.Join(err, closeErr)
	}()

	pool, err := conn.LookupStoragePoolByName(opts.pool)
	if err != nil {
		return fmt.Errorf("lookup storage pool %q: %w", opts.pool, err)
	}
	defer func() { err = errors.Join(err, pool.Free()) }()

	volume, err := pool.LookupVolumeByName(opts.volume)
	if err != nil {
		return fmt.Errorf("lookup volume %q: %w", opts.volume, err)
	}
	defer func() { err = errors.Join(err, volume.Free()) }()

	stream, err := conn.NewStream(uint32(opts.streamFlags))
	if err != nil {
		return fmt.Errorf("create stream: %w", err)
	}
	active := false
	defer func() {
		if active {
			err = errors.Join(err, stream.Abort())
		}
		err = errors.Join(err, stream.Free())
	}()

	flags := uint32(opts.flags)
	var transferred int64
	switch opts.direction {
	case "upload":
		if err := volume.Upload(stream, opts.offset, opts.length, flags); err != nil {
			return fmt.Errorf("start upload: %w", err)
		}
		active = true
		transferred, err = upload(stream, opts.file, opts.length)
	case "download":
		if err := volume.Download(stream, opts.offset, opts.length, flags); err != nil {
			return fmt.Errorf("start download: %w", err)
		}
		active = true
		transferred, err = download(stream, opts.file, opts.length)
	}
	if err != nil {
		return err
	}
	if err := stream.Finish(); err != nil {
		return fmt.Errorf("finish %s: %w", opts.direction, err)
	}
	active = false

	fmt.Printf("%s complete: %d bytes\n", opts.direction, transferred)
	return nil
}

func upload(stream *libvirt.Stream, path string, length uint64) (written int64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open upload source: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	if length == 0 {
		written, err = io.Copy(stream, file)
	} else {
		written, err = io.CopyN(stream, file, int64(length))
	}
	if err != nil {
		return written, fmt.Errorf("upload data: %w", err)
	}
	return written, nil
}

func download(stream *libvirt.Stream, path string, length uint64) (written int64, err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open download destination: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	if length == 0 {
		written, err = io.Copy(file, stream)
	} else {
		written, err = io.CopyN(file, stream, int64(length))
	}
	if err != nil {
		return written, fmt.Errorf("download data: %w", err)
	}
	return written, nil
}
