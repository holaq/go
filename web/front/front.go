package front

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"zwei.ren/console"
	"zwei.ren/encrypt"
	"zwei.ren/file"
	"zwei.ren/file/zip"
	"zwei.ren/web"
)

var (
	isBusy  = false
	lastMD5 = []byte{}

	FrontUpdateKey string
	FrontDir       = `./`
)

func Init(server *web.HttpServer, key, frontDir string) {
	if len(frontDir) != 0 {
		FrontDir = frontDir
	}
	file.EnsureDir(FrontDir)
	FrontUpdateKey = key

	if server == nil {
		server = web.DefaultServer
	}

	server.AddGZipStaticRouter("/static/", path.Join(FrontDir, "dist", "static"), true, ".js", ".css", ".html")
	// server.AddRouter("/favicon.ico", func() web.IHandler { return new(IconHandler) })
	if len(FrontUpdateKey) != 0 {
		server.AddRouter("/vueupdate", func() web.IHandler { return new(VueUpdateHandler) })
	}
	server.AddRouter("/", func() web.IHandler { return new(HomeHandler) })
}

// type IconHandler struct {
// 	web.Handler
// }

// func (this *IconHandler) Handle() {
// 	if bs, e := file.ReadFile(path.Join(FrontDir, "dist", "favicon.ico"), 0, -1); e == nil {
// 		this.ResponseOK()
// 		this.ResponseData(bs)
// 		this.ResponseHeaders(map[string][]string{
// 			"Content-Type": []string{
// 				"image/png",
// 			},
// 		})
// 	}
// }

type HomeHandler struct {
	web.Handler
}

func (this *HomeHandler) Handle() {
	fmt.Println(console.Green("Somebody come: %v", this.GetHeaders()))
	fileName := this.GetRouterPathAt(1)
	if len(fileName) == 0 {
		fileName = "index.html"
	}

	if bs, e := file.ReadFile(path.Join(FrontDir, "dist", fileName), 0, -1); e == nil {
		this.ResponseOK()
		this.ResponseData(bs)
		this.ResponseHeaders(map[string][]string{
			"Content-Type": []string{
				mime.TypeByExtension(path.Ext(fileName)),
			},
		})
	}
}

type VueUpdateHandler struct {
	web.Handler
}

func (this *VueUpdateHandler) Handle() {
	respMsg := "未知错误"

	fileNames := []string{}
	histroyDir := path.Join(FrontDir, "history")
	fileCount := 0
	for _, n := range file.GetDirChildren(histroyDir, file.File) {
		if path.Ext(n) == ".zip" {
			fileNames = append(fileNames, n)
			fileCount++
		}
	}
	sort.Strings(fileNames)

	versions := []string{}
	var name string
	for i := fileCount - 1; i > -1 && i > fileCount-11; i-- {
		name = fileNames[i]
		versions = append(versions, name[:len(name)-4])
	}

	if isBusy {
		respMsg = "正在处理另一个操作"
	} else {
		isBusy = true
		defer func() {
			isBusy = false
		}()
		if this.Method == "get" {
			respMsg = "请传入文件或输入历史版本号"
		} else if key := this.GetPostStrParam("key"); len(key) == 0 {
			respMsg = "请输入密码"
		} else if key == FrontUpdateKey {
			tempDir := path.Join(FrontDir, "temp")
			file.DeletePath(tempDir)
			defer file.DeletePath(tempDir)

			distDir := path.Join(FrontDir, "dist")

			if f := this.GetFileParam("file"); f == nil || len(f.Content) == 0 {
				if zipUrl := this.GetPostStrParam("url"); len(zipUrl) == 0 || !strings.HasPrefix(zipUrl, "http") { // 没有url
					if version := this.GetPostStrParam("version"); len(version) == 0 || version == "空" {
						respMsg = "未传入文件或版本号"
					} else if bs, e := file.ReadFile(path.Join(histroyDir, version+".zip"), 0, -1); e == nil { // 恢复历史版本
						if e := zip.UnZipFolder(tempDir, bs, true); e == nil {
							file.DeletePath(distDir)
							if e = os.Rename(path.Join(tempDir, "dist"), distDir); e == nil {
								respMsg = "恢复版本成功"
							} else {
								respMsg = "解压后目录错误 2"
								fmt.Println("Wrong struct 2:", e)
							}
						} else {
							respMsg = "解压失败:" + e.Error()
						}
					} else {
						respMsg = "该版本不存在"
					}
				} else if zipBytes, e := HttpGet(zipUrl); e != nil { // 有url
					respMsg = "服务器下载链接失败"
				} else if e := zip.UnZipFolder(tempDir, zipBytes, true); e == nil {
					file.DeletePath(distDir)
					if e = os.Rename(path.Join(tempDir, "dist"), distDir); e == nil {
						respMsg = "更新成功"

						if e = file.EnsureDir(histroyDir); e == nil {
							versionName := time.Now().Format("20060102150405")
							if e = file.WriteFile(path.Join(histroyDir, versionName+".zip"), f.Content, true, true); e == nil {
								respMsg += ", 该版本号为:" + versionName
							}
						}
					} else {
						respMsg = "解压后目录错误 3"
						fmt.Println("Wrong struct 1:", e)
					}
				} else {
					respMsg = "解压失败:" + e.Error()
				}
			} else if md5 := encrypt.MD5(f.Content); bytes.Equal(md5, lastMD5) {
				respMsg = "和上一个任务相同的文件，不作处理"
			} else if e := zip.UnZipFolder(tempDir, f.Content, true); e == nil {
				file.DeletePath(distDir)
				if e = os.Rename(path.Join(tempDir, "dist"), distDir); e == nil {
					respMsg = "更新成功"
					lastMD5 = md5

					if e = file.EnsureDir(histroyDir); e == nil {
						versionName := time.Now().Format("20060102150405")
						if e = file.WriteFile(path.Join(histroyDir, versionName+".zip"), f.Content, true, true); e == nil {
							respMsg += ", 该版本号为:" + versionName
						}
					}
				} else {
					respMsg = "解压后目录错误 1"
					fmt.Println("Wrong struct 1:", e)
				}
			} else {
				respMsg = "解压失败:" + e.Error()
			}
		} else {
			respMsg = "错误的密码"
		}
	}
	// this.Tpl("UpdateVue.tpl", map[string]interface{}{
	// 	"msg":      respMsg,
	// 	"versions": versions,
	// })

	buff := bytes.NewBuffer([]byte{})
	if e := updateTpl.Execute(buff, map[string]interface{}{
		"msg":      respMsg,
		"versions": versions,
	}); e == nil {
		this.ResponseOK()
		this.ResponseData(buff.Bytes())
		this.ResponseHeaders(map[string][]string{
			"Content-Type": []string{
				"text/html; charset=utf-8",
			},
		})
	}
	buff.Reset()
	buff = nil
}

var (
	updateTpl *template.Template = template.Must(template.New(`VueUpdate`).Parse(`<!DOCTYPE html>
<html>

<head>
	<meta name='viewport' content='width=device-width, initial-scale=1.0, user-scalable=no'>
	<meta http-equiv='Content-Type' content='text/html;charset=utf-8'>
	<title>Vue更新</title>
	<script type='text/javascript'>
	window.onload = function() { document.getElementById('u').value = window.localStorage.getItem('_U') }
	</script>
</head>

<body>
	<form enctype='multipart/form-data' action='?' method='POST'>
		<p>密码: </p>
		<p>
			<input type='password' name='key' value=''>
		</p>
		<p>dist压缩文件: </p>
		<p>
			<input type='file' name='file' value=''>
		</p>
		<p>dist压缩文件链接: </p>
		<p>
			<input type='text' name='url' value=''>
		</p>
		<p>恢复历史版本: </p>
		<p>
			<select type='text' name='version' value=''>
				{{range $i, $v := .versions}}
				<option value="{{$v}}">{{$v}}</option>
				{{end}}
			</select>
		</p>
		<input type='submit' value='提交' /> </form>

		<div style="margin-top: 20px;">
			{{.msg}}
		</div>
</body>

</html>`))
)

func HttpGet(url string) (bs []byte, e error) {
	var httpReq *http.Request
	if httpReq, e = http.NewRequest("GET", url, nil); e == nil {
		var httpRes *http.Response
		client := new(http.Client)
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		if httpRes, e = client.Do(httpReq); e == nil {
			defer httpRes.Body.Close()
			if httpRes.StatusCode == 200 {
				bs, e = ioutil.ReadAll(httpRes.Body)
			} else {
				e = errors.New("Status code: " + strconv.Itoa(httpRes.StatusCode))
			}
		}
	}
	return
}
