package zip

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"zwei.ren/file"
)

func UnZip(zipBytes []byte) (files map[string][]byte, e error) {
	var zipReader *zip.Reader
	if zipReader, e = zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes))); e == nil {

		files = make(map[string][]byte, len(zipReader.File))
		var bs []byte
		var channel io.ReadCloser
		for _, entry := range zipReader.File {
			if info := entry.FileInfo(); !info.IsDir() {
				channel, e = entry.Open()
				if e == nil {
					bs, e = ioutil.ReadAll(channel)
					files[entry.Name] = bs
				}
				channel.Close()
				if e != nil {
					return
				}
			}
		}
	}
	return
}

func UnZipFolder(toDir string, zipBytes []byte, autoMkdir bool) (e error) {
	var zipReader *zip.Reader
	if zipReader, e = zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes))); e == nil {
		for _, entry := range zipReader.File {
			filePath := path.Join(toDir, entry.Name)
			if info := entry.FileInfo(); info.IsDir() {
				if e = file.EnsureDir(filePath); e != nil {
					return
				}
			} else {
				if autoMkdir {
					if e = file.EnsureDir(path.Dir(filePath)); e != nil {
						return
					}
				}
				var f *os.File
				if f, e = os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0777); e == nil {
					defer f.Close()
					var channel io.ReadCloser
					if channel, e = entry.Open(); e == nil {
						defer channel.Close()
						if _, e = io.Copy(f, channel); e != nil {
							return
						}
					} else {
						return
					}
				} else {
					return
				}
			}
		}
	}
	return
}

func ZipFolder(folder string, showTopName bool, filter file.FileFilter) (zipBytes []byte, err error) {
	if !file.IsDirExists(folder) {
		err = errors.New(folder + "is not a folder")
		return
	}

	if filter == nil {
		filter = file.FileFilterAllAccept
	}
	if !path.IsAbs(folder) {
		if folder, err = filepath.Abs(folder); err != nil {
			return
		}
	}
	folderLen := len(folder)
	if folderLen == 0 {
		err = errors.New("Cannot read root path")
		return
	}
	if folder[folderLen-1] != '/' { // 没有带 /，待会会有的
		folderLen++
	}
	if showTopName {
		folderLen -= (len(path.Base(folder)) + 1)
	}

	queueFiles := []string{}
	const maxFileCount = 1024 * 1024 // 1G / 1k
	fileCount := 0
	file.GetDirChildrenWithFilter(folder, -1, func(fpath, name, suffix string, isFile bool, size, modTime int64, level int) bool {
		if fileCount > maxFileCount {
			return false
		}
		if filter(fpath, name, suffix, isFile, size, modTime, level) {
			// fmt.Println(fpath)
			if isFile {
				queueFiles = append(queueFiles, fpath)
				fileCount++
			}
			return true
		} else {
			return false
		}
	})
	if fileCount > maxFileCount {
		err = errors.New("Too many files")
	}

	zipMap := make(map[string][]byte, fileCount)
	for _, filePath := range queueFiles {
		if zipMap[filePath[folderLen:]], err = file.ReadFile(filePath, 0, -1); err != nil {
			return
		}
	}

	return ZipCompress(zipMap)
}

func ZipCompress(name2Bytes map[string][]byte) (zipBytes []byte, err error) {
	buff := bytes.NewBuffer([]byte{})
	zipWriter := zip.NewWriter(buff)
	defer zipWriter.Close()
	var zipWriterItem io.Writer
	for name, bs := range name2Bytes {
		if zipWriterItem, err = zipWriter.Create(name); err == nil {
			if _, err = zipWriterItem.Write(bs); err == nil {
				zipWriter.Flush()
			} else {
				return
			}
		} else {
			return
		}
	}
	zipWriter.Close()
	zipBytes = buff.Bytes()
	buff.Reset()
	buff = nil
	return
}
