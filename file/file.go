package file

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

func EnsureDir(dir string) error {
	os.MkdirAll(dir, 0777)
	info, err := os.Stat(dir)
	if err == nil && info.IsDir() {
		return nil
	} else {
		return err
	}
}

func IsDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func IsFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func DeletePath(path string) int {
	count := 0
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			if path[len(path)-1] != '/' {
				path += "/"
			}
			for _, childName := range GetDirChildren(path, All) {
				count += DeletePath(path + childName)
			}
		}
		os.Remove(path)
		count++
	}
	return count
}

func WriteFile(path string, data []byte, createIfNoFile bool, overrideOrAppend bool) error {

	var file *os.File
	var err error
	file, err = openFile(path, createIfNoFile, false, overrideOrAppend)
	if err != nil {
		return err
	}
	defer file.Close()
	var count int
	count, err = file.Write(data)
	if err != nil {
		return err
	}
	if count != len(data) {
		return errors.New(fmt.Sprintf("Wanna write %d bytes but actually write %d", len(data), count))
	}
	return nil
}

func ReadFile(path string, offset, leng int64) (content []byte, err error) {
	var file *os.File
	file, err = openFile(path, false, true, false)
	if err != nil {
		return
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	buffLen := fileInfo.Size() - offset
	if leng > 0 && leng < buffLen {
		buffLen = leng
	}
	if buffLen < 0 {
		return content, errors.New("Read length < 0")
	}
	content = make([]byte, buffLen)
	file.ReadAt(content, offset)
	return
}

func openFile(path string, createIfNoFile bool, read bool, overrideOrAppend bool) (*os.File, error) {
	var flag int
	if read {
		flag = os.O_RDONLY
	} else {
		flag = os.O_WRONLY

		if createIfNoFile {
			flag = flag | os.O_CREATE
		}
		if overrideOrAppend {
			os.Remove(path)
		} else {
			flag = flag | os.O_APPEND
		}
	}
	return os.OpenFile(path, flag, 0666)
}

const (
	All  int = 0
	File int = 1
	Dir  int = 2
)

// 0,all kind
// 1.only file
// 2.only dir
func GetDirChildren(dirPath string, flag int) []string {
	var files = []string{}

	infos, err := ioutil.ReadDir(dirPath)

	files = make([]string, 0, len(infos))
	if err == nil {
		for _, info := range infos {
			if flag != All {
				if info.IsDir() {
					if flag == File {
						continue
					}
				} else {
					if flag == Dir {
						continue
					}
				}
			}
			files = append(files, info.Name())
		}
	}
	return files
}

func GetExecFileDir() (dir string, err error) {
	return filepath.Abs(filepath.Dir(os.Args[0]))
}

func CloneFile(from, to string) (e error) {
	var f *os.File
	if f, e = os.OpenFile(from, os.O_RDONLY, 0666); e == nil {
		defer f.Close()
		var t *os.File
		if t, e = os.OpenFile(to, os.O_CREATE|os.O_WRONLY, 0666); e == nil {
			defer t.Close()
			_, e = io.Copy(t, f)
		}
	}
	return
}

// 将from文件夹下面的文件，全部clone到toDir，例如传入./a/, ./b/, 则 ./a/c.txt 会clone到 ./b/c.txt
func CloneDir(fromDir, toDir string, replaceSame bool, filter func(filePath string) bool) (targetFiles []string, e error) {
	if filter == nil {
		filter = func(f string) bool {
			return true
		}
	}
	var info os.FileInfo
	if info, e = os.Stat(fromDir); e == nil {
		if info.IsDir() { // dir
			targetFiles = []string{}
			var okFiles []string
			if e = EnsureDir(toDir); e == nil {
				for _, fileName := range GetDirChildren(fromDir, All) {
					okFiles, e = CloneDir(path.Join(fromDir, fileName), path.Join(toDir, fileName), replaceSame, filter)
					targetFiles = append(targetFiles, okFiles...)
					if e != nil {
						break
					}
				}
			}
		} else { // file
			if replaceSame || !IsFileExists(toDir) && filter(fromDir) {
				if e = CloneFile(fromDir, toDir); e == nil {
					targetFiles = []string{toDir}
				}
				// var bs []byte
				// if bs, e = ReadFile(fromDir, 0, -1); e == nil {
				// 	if e = WriteFile(toDir, bs, true, true); e == nil {
				// 		targetFiles = []string{toDir}
				// 	}
				// }
			}
		}
	}

	if e != nil {
		for _, file := range targetFiles {
			os.Remove(file)
		}
		targetFiles = []string{}
	}
	return
}

type FileInfo struct {
	Path, Name, Suffix string
	IsFile             bool
	Size, ModTime      int64
	Level              int
}
type FileInfoFilter func(FileInfo) bool

func WalkFiles(dirPath string, maxLevel int, filter FileInfoFilter) FileTree {
	_fileter := func(path, name, suffix string, isFile bool, size, modTime int64, level int) bool {
		return filter(FileInfo{
			Path:    path,
			Name:    name,
			Suffix:  suffix,
			IsFile:  isFile,
			Size:    size,
			ModTime: modTime,
			Level:   level,
		})
	}
	return GetDirChildrenWithFilter(dirPath, maxLevel, _fileter)
}

type FileFilter func(path, name, suffix string, isFile bool, size, modTime int64, level int) bool
type FileTree map[string]FileTree

func (this FileTree) GetFiles() []string {
	res := []string{}
	if this != nil && len(this) != 0 {
		for pre, t := range this {
			if t == nil {
				res = append(res, pre)
			} else {
				for _, s := range t.GetFiles() {
					res = append(res, path.Join(pre, s))
				}
			}
		}
	}
	return res
}

var FileFilterAllAccept FileFilter = func(path, name, suffix string, isFile bool, size, modTime int64, level int) bool { return true }

func GetDirChildrenWithFilter(dirPath string, maxLevel int, filter FileFilter) FileTree {
	if filter == nil {
		filter = FileFilterAllAccept
	}
	if maxLevel < 0 {
		maxLevel = 0xffff
	}
	return _getDirChildrenWithFilter(dirPath, 0, maxLevel, filter)
}

// 传入的dirpath必须为文件夹，level从0开始
func _getDirChildrenWithFilter(dirPath string, level, maxLevel int, filter FileFilter) FileTree {
	if level < maxLevel {
		// name, ext := path.Base(dirPath), path.Ext(dirPath)
		infos, err := ioutil.ReadDir(dirPath)
		if len(infos) != 0 {
			var fpath, name string
			if err == nil {
				tree := FileTree{}
				var _t FileTree
				for _, info := range infos {
					name = info.Name()
					fpath = path.Join(dirPath, name)
					if info.IsDir() {
						if filter(fpath, name, "", false, -1, info.ModTime().Unix(), level) {
							if _t = _getDirChildrenWithFilter(fpath, level+1, maxLevel, filter); _t == nil {
								tree[name] = FileTree{}
							} else {
								tree[name] = _t
							}
						}
					} else {
						if filter(fpath, name, path.Ext(name), true, info.Size(), info.ModTime().Unix(), level) {
							tree[name] = nil
						}
					}
				}
				return tree
			}
		}
	}
	return nil
}

type FolderStruct struct {
	Name     string         `json:"name,omitempty"`
	IsDir    bool           `json:"isDir,omitempty"`
	Size     int64          `json:"size,omitempty"`
	ModifyAt int64          `json:"modifyAt,omitempty"`
	Children []FolderStruct `json:"children,omitempty"`
}

func GetFolderStruct(dirPath string, maxLevel int) FolderStruct {
	if maxLevel < 0 {
		maxLevel = 0xffff
	}
	s, _ := _getFolderStruct(dirPath, 0, maxLevel)
	return s
}

// 传入的dirpath必须为文件夹，level从0开始
func _getFolderStruct(dirPath string, level, maxLevel int) (s FolderStruct, ok bool) {
	if level < maxLevel {
		// name, ext := path.Base(dirPath), path.Ext(dirPath)
		if info, e := os.Stat(dirPath); e == nil {
			ok = true
			s.Name = info.Name()
			s.Size = info.Size()
			s.ModifyAt = info.ModTime().Unix()
			if s.IsDir = info.IsDir(); s.IsDir {
				var infos []os.FileInfo
				if infos, e = ioutil.ReadDir(dirPath); e == nil {
					if childrenCount := len(infos); childrenCount != 0 {
						s.Children = make([]FolderStruct, childrenCount)
						var _s FolderStruct
						var _z bool
						var _i int
						for _i, info = range infos {
							if _s, _z = _getFolderStruct(path.Join(dirPath, info.Name()), level+1, maxLevel); _z {
								s.Children[_i] = _s
							} else {
								s.Children[_i] = FolderStruct{Name: info.Name()}
							}
						}
					}

				}

			}
		}
	}
	return
}

// func UnZip(zipBytes []byte) (files map[string][]byte, e error) {
// 	var zipReader *zip.Reader
// 	if zipReader, e = zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes))); e == nil {

// 		files = make(map[string][]byte, len(zipReader.File))
// 		var bs []byte
// 		var channel io.ReadCloser
// 		for _, entry := range zipReader.File {
// 			if info := entry.FileInfo(); !info.IsDir() {
// 				channel, e = entry.Open()
// 				if e == nil {
// 					bs, e = ioutil.ReadAll(channel)
// 					files[entry.Name] = bs
// 				}
// 				channel.Close()
// 				if e != nil {
// 					return
// 				}
// 			}
// 		}
// 	}
// 	return
// }

// func UnZipFolder(toDir string, zipBytes []byte) (e error) {
// 	var zipReader *zip.Reader
// 	if zipReader, e = zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes))); e == nil {
// 		for _, entry := range zipReader.File {
// 			filePath := path.Join(toDir, entry.Name)
// 			if info := entry.FileInfo(); info.IsDir() {
// 				if e = EnsureDir(filePath); e != nil {
// 					return
// 				}
// 			} else {
// 				var f *os.File
// 				if f, e = os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0777); e == nil {
// 					defer f.Close()
// 					var channel io.ReadCloser
// 					if channel, e = entry.Open(); e == nil {
// 						defer channel.Close()
// 						if _, e = io.Copy(f, channel); e != nil {
// 							return
// 						}
// 					} else {
// 						return
// 					}
// 				} else {
// 					return
// 				}
// 			}
// 		}
// 	}
// 	return
// }

// func ZipFolder(folder string, showTopName bool, filter FileFilter) (zipBytes []byte, err error) {
// 	if !IsDirExists(folder) {
// 		err = errors.New(folder + "is not a folder")
// 		return
// 	}

// 	if filter == nil {
// 		filter = FileFilterAllAccept
// 	}
// 	if !path.IsAbs(folder) {
// 		if folder, err = filepath.Abs(folder); err != nil {
// 			return
// 		}
// 	}
// 	folderLen := len(folder)
// 	if folderLen == 0 {
// 		err = errors.New("Cannot read root path")
// 		return
// 	}
// 	if folder[folderLen-1] != '/' { // 没有带 /，待会会有的
// 		folderLen++
// 	}
// 	if showTopName {
// 		folderLen -= (len(path.Base(folder)) + 1)
// 	}

// 	queueFiles := []string{}
// 	const maxFileCount = 1024 * 1024 // 1G / 1k
// 	fileCount := 0
// 	GetDirChildrenWithFilter(folder, -1, func(fpath, name, suffix string, isFile bool, size, modTime int64, level int) bool {
// 		if fileCount > maxFileCount {
// 			return false
// 		}
// 		if filter(fpath, name, suffix, isFile, size, modTime, level) {
// 			fmt.Println(fpath)
// 			if isFile {
// 				queueFiles = append(queueFiles, fpath)
// 				fileCount++
// 			}
// 			return true
// 		} else {
// 			return false
// 		}
// 	})
// 	if fileCount > maxFileCount {
// 		err = errors.New("Too many files")
// 	}

// 	zipMap := make(map[string][]byte, fileCount)
// 	for _, filePath := range queueFiles {
// 		if zipMap[filePath[folderLen:]], err = ReadFile(filePath, 0, -1); err != nil {
// 			return
// 		}
// 	}

// 	return ZipCompress(zipMap)
// }
