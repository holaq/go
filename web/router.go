package web

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"zwei.ren/console"
	"zwei.ren/log"
	"zwei.ren/memory"
)

const (
	boundary_find = "boundary="
)

var (
	boundary_findlen    = len(boundary_find)
	pat_multipart_name  *regexp.Regexp
	pat_multipart_fname *regexp.Regexp
	pat_multipart_ctype *regexp.Regexp

	tplFolder = "./views"

	Err_Abort = errors.New("Abort by you")

	// routers = []*_Router{}

	MaxStaticFileSize int64 = 1024 * 1024 * 10 // 10m
	StreamBuffSize          = 4096
	ReadTimeout             = time.Second * 600
	WriteTimeout            = time.Second * 600

	DefaultServer = &HttpServer{IsLog: true}

	XmlParser func([]byte) (map[string]interface{}, error) = nil

	err_UnknownCType           = errors.New("Unknown Content Type")
	Err_HandleMetUnimplemented = errors.New("The handle method is not implemented")
)

type HttpServer struct {
	IsLog bool

	routers []*_Router
	mux     *http.ServeMux
	server  *http.Server
}

func (this *HttpServer) Close() error {
	if this.server == nil {
		return errors.New("Not a running server")
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if e := this.server.Shutdown(ctx); e == nil {
		this.server = nil
		return nil
	} else {
		return e
	}
}

type _Router struct {
	Name    string
	Len     int
	Builder func() IHandler
}

func init() {
	initMime()
	initRouter()
}

func initRouter() (err error) {
	if pat_multipart_name, err = regexp.Compile(`name="(.+)"\r\n`); err == nil {
		if pat_multipart_fname, err = regexp.Compile(`name="(.+)"; filename="(.+)"\r\n`); err == nil {
			pat_multipart_ctype, err = regexp.Compile(`Content-Type:\s(.+)\r\n`)
		}
	}
	return
}

type MultipartFileData struct {
	Content     []byte
	FileName    string
	ContentType string
}

type IHandler interface {
	initHandler(w http.ResponseWriter, r *http.Request)
	GetIO() (*http.Request, http.ResponseWriter)
	Prepare()
	Handle()
	IP() string
	isOver() bool
	getResponse() (statusCode int, resHeaders map[string][]string, resData interface{})

	ResponseNothing() bool
	ResponseOK()
	ResponseStatus(code int)
	ResponseHeaders(headers map[string][]string)
	ResponseData(data interface{})

	// OnConnect()
	// OnDisconnect(error)
	// OnMessage(msgType int, body []byte) error
}

type Handler struct {
	Writer  http.ResponseWriter
	Request *http.Request

	isFinish, isNoResponse bool
	body                   []byte
	hasBody                bool
	getParams              map[string]string
	hasGetParams           bool
	postParams             map[string]interface{}
	hasPostParams          bool
	fileParams             map[string]*MultipartFileData
	reqHeaders             map[string][]string
	ip                     *string
	hasReqHeaders          bool
	Method                 string

	ResCode    int
	ResHeaders map[string][]string
	ResData    interface{}
}

func (this *Handler) Tpl(fileName string, datas interface{}, subTpls ...string) (e error) {
	var tpl *template.Template
	subCount := 0
	if subTpls != nil {
		subCount = len(subTpls)
	}
	tplFiles := make([]string, 1+subCount)
	tplFiles[0] = path.Join(tplFolder, fileName)
	for i := 0; i < subCount; i++ {
		tplFiles[i+1] = path.Join(tplFolder, subTpls[i])
	}
	if tpl, e = template.ParseFiles(tplFiles...); e == nil {
		buff := bytes.NewBuffer([]byte{})
		if e = tpl.Execute(buff, datas); e == nil {
			this.ResponseOK()
			this.ResponseData(buff.Bytes())
		}
		buff.Reset()
		buff = nil
	}
	if e != nil {
		log.Error("Router handle tpl failed: req[%s] fn[%s] datas[%v] error[%v]", this.Request.URL.Path, fileName, datas, e)
	}
	return
}

func (this *Handler) GetRouterPath() []string {
	return strings.Split(this.Request.URL.Path, "/")
}
func (this *Handler) GetRouterPathAt(index int) string {
	paths := strings.Split(this.Request.URL.Path, "/")
	if index < len(paths) {
		return paths[index]
	} else {
		return ""
	}
}
func (this *Handler) initHandler(w http.ResponseWriter, r *http.Request) {
	this.Writer = w
	this.Request = r
	this.Method = strings.ToLower(r.Method)
	this.ResCode = 404
}
func (this *Handler) GetIO() (*http.Request, http.ResponseWriter) {
	return this.Request, this.Writer
}
func (this *Handler) isOver() bool {
	return this.isFinish
}
func (this *Handler) StopRun() {
	this.isFinish = true
	panic(Err_Abort)
}
func (this *Handler) NoResponse() {
	this.isNoResponse = true
}
func (this *Handler) ResponseNothing() bool {
	return this.isNoResponse
}
func (this *Handler) WriteString(msg string) {
	this.Writer.Write([]byte(msg))
}
func (this *Handler) SetStatus(status int) {
	this.Writer.WriteHeader(status)
}
func (this *Handler) Prepare() {}
func (this *Handler) Handle() {
	panic(Err_HandleMetUnimplemented)
}
func (this *Handler) GetHeaderArr(key string) []string {
	arr, exists := this.GetHeaders()[key]
	if !exists {
		arr, _ = this.GetHeaders()[strings.ToLower(key)]
	}
	return arr
}
func (this *Handler) GetHeader(key string) string {
	if arr := this.GetHeaderArr(key); arr == nil || len(arr) == 0 {
		return ""
	} else {
		return arr[0]
	}
}
func (this *Handler) GetHeaders() map[string][]string {
	if !this.hasReqHeaders {
		this.reqHeaders = map[string][]string{}
		this.hasReqHeaders = true
		for k, vs := range this.Request.Header {
			if len(k) == 0 || len(vs) == 0 {
				continue
			}
			this.reqHeaders[strings.ToLower(k)] = vs
		}
	}
	return this.reqHeaders
}
func (this *Handler) GetGetParam(key string) (res string) {
	return this.GetGetParamMap()[key]
}
func (this *Handler) GetGetParamMap() map[string]string {
	if !this.hasGetParams {
		this.getParams = map[string]string{}
		this.hasGetParams = true
		if getForm, err := url.ParseQuery(this.Request.URL.RawQuery); err == nil {
			for k, vs := range getForm {
				if len(k) != 0 && len(vs) != 0 && len(vs[0]) != 0 {
					this.getParams[k] = vs[0]
				}
			}
		}
	}
	return this.getParams
}

func (this *Handler) GetFileParams() map[string]*MultipartFileData {
	if !this.hasPostParams {
		this.GetPostParams()
	}
	return this.fileParams
}

func (this *Handler) getRemoteAddr() string {
	return this.Request.RemoteAddr
}

func (this *Handler) GetFileParam(key string) (f *MultipartFileData) {
	f, _ = this.GetFileParams()[key]
	return
}

func (this *Handler) GetPostParams() map[string]interface{} {
	if !this.hasPostParams {
		params, fileParams := map[string]interface{}{}, map[string]*MultipartFileData{}
		var e error
		if cType := this.GetHeader(`content-type`); len(cType) > 15 {
			switch cType[:16] {
			case "application/x-ww":
				var form url.Values
				if form, e = url.ParseQuery(string(this.GetBody())); e == nil {
					for k, vs := range form {
						if len(k) != 0 && len(vs) != 0 {
							params[k] = vs[0]
						}
					}
				}
			case "application/json":
				e = json.Unmarshal(this.GetBody(), &params)
			case "multipart/form-d":
				if bInd := strings.Index(cType, boundary_find); bInd != -1 {
					params, fileParams = parseMultipartForm(this.GetBody(), cType[bInd+boundary_findlen:])
				}
			default:
				e = err_UnknownCType
				log.Debug("This Content-Type: [%v] doesn't need to be parsed", cType)
			}
		} else {
			switch cType {
			case "text/xml":
				var xmlBody map[string]interface{}
				if XmlParser == nil {
					log.Error("XML isn't supported")
				} else if xmlBody, e = XmlParser(this.GetBody()); e != nil {
					log.Error("XML parser return failed: %v", e)
				} else {
					params = xmlBody
				}
			default:
				e = err_UnknownCType
			}
		}
		if e == err_UnknownCType {
			if json.Unmarshal(this.GetBody(), &params) != nil {
				if form, err := url.ParseQuery(string(this.GetBody())); err == nil {
					for k, vs := range form {
						if len(k) != 0 && len(vs) != 0 {
							params[k] = vs[0]
						}
					}
				}
			}
		}

		// fmt.Printf("ConType: [%s] Err: [%v] Form: [%+v]\n",
		// 	this.GetHeader(`content-type`), e, params,
		// )

		this.postParams = params
		this.fileParams = fileParams

		this.hasPostParams = true
	}
	return this.postParams
}

func (this *Handler) GetPostStrParam(key string) (value string) {
	val := this.GetPostParam(key)
	var isStr bool
	if value, isStr = val.(string); !isStr {
		value = fmt.Sprintf("%v", val)
	}
	return
}

func (this *Handler) GetPostIntParam(key string, defaultVal int) int {
	if i, z := this.GetPostParams()[key]; z {
		switch v := i.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		case string:
			if r, e := strconv.Atoi(v); e == nil {
				return r
			}
		default:
			log.Error("Type of params[%s] is %T", key, i)
		}
	}
	return defaultVal
}

func (this *Handler) GetPostParam(key string) (value interface{}) {
	value, _ = this.GetPostParams()[key]
	return
}

func (this *Handler) ParseParams(pointer interface{}) error {
	return json.Unmarshal(this.GetBody(), pointer)
}

func (this *Handler) GetBody() (body []byte) {
	if !this.hasBody {
		this.hasBody = true
		this.body, _ = ioutil.ReadAll(this.Request.Body)
		this.Request.Body.Close()
	}
	return this.body
}

func (this *Handler) IP() (ip string) {
	if this.ip == nil {
		if ip = this.GetHeader("x-forwarded-for"); len(ip) == 0 {
			ip = this.Request.RemoteAddr
			if ind := strings.LastIndex(ip, ":"); ind != -1 {
				ip = ip[:ind]
			}
		}
		this.ip = &ip
	}
	return
}

func (this *Handler) ResponseStatus(code int) {
	this.ResCode = code
}
func (this *Handler) ResponseHeaders(headers map[string][]string) {
	this.ResHeaders = headers
}
func (this *Handler) ResponseData(data interface{}) {
	this.ResData = data
}
func (this *Handler) ResponseOK() {
	this.ResponseStatus(200)
}

func (this *Handler) ResponseAccessCrossOrigin(headers map[string][]string, allowHeaders, exposeHeaders []string) {
	allows := "Content-Type, Content-Length, Authorization, Accept, X-Requested-With"
	if allowHeaders != nil {
		allows = strings.Join(append(allowHeaders, allows), ", ")
	}
	exposes := ""
	if exposeHeaders != nil {
		exposes = strings.Join(exposeHeaders, ", ")
	}

	m := map[string][]string{
		"Access-Control-Allow-Origin":      []string{"*"},
		"Access-Control-Allow-Methods":     []string{"PUT, POST, GET, DELETE, OPTIONS"},
		"Access-Control-Allow-Headers":     []string{allows},
		"Access-Control-Expose-Headers":    []string{exposes},
		"Access-Control-Max-Age":           []string{"1728000"},
		"Access-Control-Allow-Credentials": []string{"true"},
	}
	if headers != nil {
		for k, vs := range headers {
			m[k] = vs
		}
	}
	this.ResponseHeaders(m)
}

func (this *Handler) getResponse() (int, map[string][]string, interface{}) {
	return this.ResCode, this.ResHeaders, this.ResData
}

func HandleException(msg string) {
	if err := recover(); err != nil {
		if err == Err_Abort {
			fmt.Println("Abort")
		} else {
			log.Error("Panic!!!!! at[%v]: %v\nTrace: %v", msg, err, memory.PanicTrace(10))
		}
	}
}

func AddFileRouter(routerName, folder string, filter func([]string, os.FileInfo) bool) {
	DefaultServer.AddFileRouter(routerName, folder, filter)
}
func (this *HttpServer) AddFileRouter(routerName, folder string, filter func([]string, os.FileInfo) bool) {
	this.AddRouter(routerName, func() IHandler {
		return &staticRouter{Folder: folder, NoFilter: filter == nil, Filter: filter}
	})
}

func AddGZipStaticRouter(routerName, folder string, openCache bool, suffixes ...string) {
	DefaultServer.AddGZipStaticRouter(routerName, folder, openCache, suffixes...)
}

func (this *HttpServer) AddGZipStaticRouter(routerName, folder string, openCache bool, suffixes ...string) {
	isGZip := suffixes != nil && len(suffixes) != 0
	sufMap := map[string]bool{}
	if isGZip {
		for _, s := range suffixes {
			if len(s) != 0 && s[0] == '.' {
				sufMap[s] = true
			}
		}
		isGZip = len(sufMap) != 0
	}
	this.AddRouter(routerName, func() IHandler {
		return &staticRouter{
			Folder:       folder,
			NoFilter:     true,
			GZip:         isGZip,
			GZipSuffixes: sufMap,
			OpenCache:    openCache,
		}
	})
}

func AddStaticRouter(routerName, folder string) {
	DefaultServer.AddStaticRouter(routerName, folder)
}

func (this *HttpServer) AddStaticRouter(routerName, folder string) {
	this.AddRouter(routerName, func() IHandler {
		return &staticRouter{Folder: folder, NoFilter: true}
	})
}

func SetTplDir(folder string) {
	tplFolder = folder
}

type staticRouter struct {
	Handler
	Folder       string
	NoFilter     bool
	GZip         bool
	GZipSuffixes map[string]bool
	Filter       func([]string, os.FileInfo) bool
	OpenCache    bool
}

func (this *staticRouter) Handle() {
	if paths := this.GetRouterPath(); len(paths) > 1 {
		paths[1] = this.Folder
		filePath := path.Join(paths[1:]...)
		info, e := os.Stat(filePath)
		if e == nil {
			if info.IsDir() {
				if path := this.Request.URL.Path; path[len(path)-1] != '/' {
					this.Redirect(302, path+"/")
					return
				}
				filePath = path.Join(filePath, "index.html")
				info, e = os.Stat(filePath)
			}
			if e == nil {
				if this.NoFilter || this.Filter(paths[2:], info) {
					size := info.Size()

					if this.ResHeaders == nil {
						this.ResHeaders = map[string][]string{
							"Content-Type": {mime.TypeByExtension(path.Ext(filePath))},
						}
					}

					if this.OpenCache {
						switch path.Ext(filePath) {
						case ".mp3", ".wav", ".avi", ".mp4":
						default:
							etag := `"` + strconv.FormatInt(info.ModTime().Unix(), 16) + "-" + strconv.FormatInt(size, 16) + `"`
							lastModify := info.ModTime().Format(http.TimeFormat)

							_lm, _etag := this.GetHeader("if-modified-since"), this.GetHeader("if-none-match")

							this.ResHeaders["ETag"] = []string{etag}
							this.ResHeaders["Last-Modified"] = []string{lastModify}

							if len(_lm) != 0 || len(_etag) != 0 { // 浏览器有缓存机制
								if (len(_lm) == 0 || _lm == lastModify) &&
									(len(_etag) == 0 || _etag == etag) { // 都命中了
									this.ResponseStatus(http.StatusNotModified)
									return
								}
							}
						}
					}

					var rangeBytesFrom, rangeBytesTo int64 = -1, -1
					if bytesRange := this.GetHeader("range"); len(bytesRange) > 7 && bytesRange[:6] == "bytes=" {
						fmt.Println("Range:", bytesRange)
						bytesRange = bytesRange[6:]
						if ind := strings.Index(bytesRange, ","); ind != -1 {
							bytesRange = bytesRange[:ind]
						}
						var ind int64
						var _e error
						if len(bytesRange) != 0 {
							if bytesRange[0] == '-' {
								if ind, _e = strconv.ParseInt(bytesRange, 10, 64); _e == nil && ind+size > -1 {
									rangeBytesFrom, rangeBytesTo = size+ind-1, size-1
								}
							} else if from_to := strings.Split(bytesRange, "-"); len(from_to) == 2 {
								if ind, _e = strconv.ParseInt(from_to[0], 10, 64); _e == nil && ind > -1 {
									if len(from_to[1]) == 0 {
										rangeBytesFrom, rangeBytesTo = ind, size-1
									} else if rangeBytesTo, _e = strconv.ParseInt(from_to[1], 10, 64); _e == nil && rangeBytesTo < size && rangeBytesTo >= ind {
										rangeBytesFrom = ind
									}
								}
							}
							if rangeBytesTo-rangeBytesFrom > MaxStaticFileSize {
								rangeBytesFrom = -1
							} else if rangeBytesFrom == 0 && rangeBytesTo == size-1 {
								rangeBytesFrom = -1
							}
						}
					}

					var bs []byte
					if rangeBytesFrom != -1 {
						var file *os.File
						if file, e = os.OpenFile(filePath, os.O_RDONLY, 0666); e == nil {
							if _, e = file.Seek(rangeBytesFrom, os.SEEK_SET); e == nil {
								rangeCount := int(rangeBytesTo - rangeBytesFrom + 1)
								readCount := 0
								buff := make([]byte, rangeCount)
								var _tempRc int
								for e == nil && readCount < rangeCount {
									if _tempRc, e = file.Read(buff[readCount:]); _tempRc > 0 {
										readCount += _tempRc
									}
								}
								if readCount == rangeCount {
									if e == io.EOF {
										e = nil
									}
									this.ResHeaders["Content-Length"] = []string{
										strconv.Itoa(rangeCount),
									}
									this.ResHeaders["Content-Range"] = []string{
										fmt.Sprintf("bytes %d-%d/%d",
											rangeBytesFrom, rangeBytesTo, size,
										),
									}
									this.ResponseData(buff)
									this.ResponseStatus(206)
								} else {
									e = errors.New("File changed")
								}

							}
							file.Close()
						}
					} else if size > MaxStaticFileSize { // 文件太大了
						var file *os.File
						if file, e = os.OpenFile(filePath, os.O_RDONLY, 0666); e == nil {
							this.ResponseData(&Stream{Reader: file})
							this.ResponseOK()
							this.ResHeaders["Content-Length"] = []string{
								strconv.FormatInt(size, 10),
							}
						}
					} else if bs, e = ioutil.ReadFile(filePath); e == nil {
						this.ResponseOK()
						if this.GZip && this.GZipSuffixes[path.Ext(this.Request.RequestURI)] {
							acceptGZip := false
							for _, s := range strings.Split(this.GetHeader("accept-encoding"), ",") {
								if strings.TrimSpace(s) == "gzip" {
									acceptGZip = true
									break
								}
							}
							if acceptGZip {
								compressed := GZipCompress(bs)
								if len(compressed) < len(bs) {
									this.ResHeaders["Content-Encoding"] = []string{"gzip"}
									this.ResponseData(compressed)
									return
								}
							}
						}
						this.ResHeaders["Content-Length"] = []string{
							strconv.FormatInt(size, 10),
						}
						this.ResponseData(bs)
					}
					if e != nil {
						log.Error("Router handle static failed: req[%s] read error[%v]", this.Request.URL.Path, e)
					}
				} else {
					fmt.Printf("FilePath[%s] filter wrong\n", filePath)
				}
			}
		} else {
			fmt.Printf("FilePath[%s] stat failed: %v\n", filePath, e)
		}
	}
}

type Stream struct {
	Reader   io.ReadCloser
	BuffSize int
}

func (this *Handler) Redirect(code int, url string) {
	this.ResponseHeaders(map[string][]string{
		"Location": []string{url},
	})
	this.ResponseStatus(code)
}

func checkBuilder(handlerBuilder func() IHandler) (e error) {
	defer func() {
		if _e := recover(); _e == Err_HandleMetUnimplemented {
			e = Err_HandleMetUnimplemented
		}
	}()
	handlerBuilder().Handle()
	return
}

func AddRouter(httpPath string, handlerBuilder func() IHandler) {
	DefaultServer.AddRouter(httpPath, handlerBuilder)
}

func (this *HttpServer) AddRouter(httpPath string, handlerBuilder func() IHandler) {
	if e := checkBuilder(handlerBuilder); e != nil {
		panic("Router [" + httpPath + "] wrong:" + e.Error())
	}

	if httpPath != "/" && (len(httpPath) < 2 || httpPath[0] != '/') {
		panic("Router path must starts with '/'!  > " + httpPath)
	}
	if handlerBuilder == nil {
		panic(fmt.Sprintf("Http handler of %v is nil! ", httpPath))
	} else {
		if httpPath[len(httpPath)-1] != '/' {
			httpPath += "/"
		}
		if this.routers == nil {
			this.routers = []*_Router{}
		}
		for _, r := range this.routers {
			if r.Name == httpPath {
				panic("Cannot add same router")
			}
		}
		this.routers = append(this.routers, &_Router{
			Name:    httpPath,
			Len:     len(httpPath),
			Builder: handlerBuilder,
		})
	}
}

type _RouterArr []*_Router

func (this _RouterArr) Len() int {
	return len(this)
}
func (this _RouterArr) Less(i, j int) bool {
	return this[j].Name < this[i].Name
}
func (this _RouterArr) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func RouterRunWithTimeout(port int, readTimeout, writeTimeout time.Duration) error {
	return DefaultServer.RouterRunWithTimeout(port, readTimeout, writeTimeout)
}

func (this *HttpServer) log(
	status_ptr *int, from time.Time,
	request *http.Request,
	handler *IHandler,
) {
	var addr, met string
	if handler != nil && *handler != nil {
		addr = (*handler).IP()
	}
	if len(addr) == 0 {
		addr = request.RemoteAddr
	}
	met = request.Method

	now := time.Now()

	status := *status_ptr
	var _status, _met string
	if status < 200 || status > 399 {
		_status = console.Red(strconv.Itoa(status))
	} else {
		_status = console.Magenta(strconv.Itoa(status))
	}
	switch met {
	case "POST":
		_met = console.Yellow(met)
	case "PUT":
		_met = console.Black(met)
	case "DELETE":
		_met = console.Magenta(met)
	case "OPTIONS":
		_met = console.Cyan(met)
	default:
		_met = console.Blue(met)
	}
	fmt.Printf(
		"[Zwei.Ren/Web] %s | %3s | %12v |%21s | %5s | %s\n",
		now.Format("2006-01-02 15:04:05"),
		_status,
		now.Sub(from),
		addr,
		_met,
		console.Green(request.URL.String()),
	)
}

func (this *HttpServer) RouterRunWithTimeout(port int, readTimeout, writeTimeout time.Duration) error {
	isLog := this.IsLog
	routers := this.routers
	mux := this.mux
	server := this.server

	if routers == nil {
		routers = []*_Router{}
	}
	if mux == nil {
		mux = http.NewServeMux()
		this.mux = mux
	}
	if server == nil {
		server = &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       readTimeout,
			ReadHeaderTimeout: readTimeout,
		}
		this.server = server
	}

	sort.Sort(_RouterArr(routers))

	// for i, r := range routers {
	// 	fmt.Println(i, ">", r.Name)
	// }
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		defer HandleException(request.RequestURI)
		defer request.Body.Close()

		status := 404
		var handler IHandler
		if isLog {
			defer this.log(&status, time.Now(), request, &handler)
		}

		uri := request.URL.Path
		uriLen := len(uri)
		if uriLen == 0 {
			writer.WriteHeader(status)
			return
		}
		if uri[uriLen-1] != '/' {
			uri += "/"
			uriLen++
		}
		var rout *_Router
		for _, r := range routers {
			if uriLen >= r.Len && uri[:r.Len] == r.Name {
				rout = r
				break
			}
		}
		if rout == nil {
			writer.WriteHeader(status)
			return
		}

		handler = rout.Builder()
		handler.initHandler(writer, request)
		handler.Prepare()
		if !handler.isOver() {
			handler.Handle()
			if handler.ResponseNothing() {
				return
			}
		}

		var headers map[string][]string
		var resData interface{}
		status, headers, resData = handler.getResponse()
		if status < 1 {
			status = 405
		}

		// 指定了headers就不用自己找Content-Type了
		// 未指定的情况下，如果是interface{}就json，否则是根据mime
		isNoHeaders := headers == nil || len(headers) == 0

		writeHeader := writer.Header()
		if !isNoHeaders {
			for hk, hv := range headers {
				for _, v := range hv {
					writeHeader.Add(hk, v)
				}
			}
		}

		var writeBs []byte
		var readStream *Stream
		if resData != nil {
			isNoJson := true
			switch v := resData.(type) {
			case string:
				writeBs = []byte(v)
			case []byte:
				writeBs = v
			case *Stream:
				readStream = v
			default:
				writeBs, _ = json.Marshal(resData)
				isNoJson = false
			}
			if isNoHeaders {
				if isNoJson {
					if cType := mime.TypeByExtension(path.Ext(request.URL.Path)); len(cType) != 0 {
						writeHeader.Set(
							"Content-Type",
							cType,
						)
					}
				} else {
					writeHeader.Set(
						"Content-Type",
						"application/json",
					)
				}
			}
		}
		writer.WriteHeader(status)

		if readStream == nil {
			if writeBs == nil || len(writeBs) == 0 {
				writeBs = []byte("Unknown Response")
			}
			writer.Write(writeBs)
		} else {
			defer readStream.Reader.Close()
			buffSize := readStream.BuffSize
			if buffSize < 1 {
				buffSize = StreamBuffSize
			}
			buff := make([]byte, buffSize)
			var rc, wc int
			var e error
			for {
				if rc, e = readStream.Reader.Read(buff); rc != 0 {
					if wc, e = writer.Write(buff[:rc]); wc != rc {
						break
					}
				}
				if e != nil {
					break
				}
			}
		}
	})

	server.Handler = mux

	fmt.Println(
		console.Cyan("[Zwei.Ren/Web] Running on port ") +
			console.Magenta(strconv.Itoa(port)) +
			console.Cyan(" with ") +
			console.Blue(strconv.Itoa(len(routers))) +
			console.Cyan(" routers."),
	)
	e := server.ListenAndServe()
	fmt.Println(
		console.Red("[Zwei.Ren/Web] Stop service on port ") +
			console.Magenta(strconv.Itoa(port)) +
			console.Red(" with error: ") +
			console.Blue(e.Error()) +
			console.Red("."),
	)
	return e
}

func Run(port int) error {
	return DefaultServer.Run(port)
}

func (this *HttpServer) Run(port int) error {
	return this.RouterRunWithTimeout(port, ReadTimeout, WriteTimeout)
}

func AsyncRun(port int) chan error {
	return DefaultServer.AsyncRun(port)
}

func (this *HttpServer) AsyncRun(port int) chan error {
	listener := make(chan error)
	go func(list chan error) {
		list <- this.Run(port)
	}(listener)
	return listener
}

func parseMultipartForm(body []byte, boundary string) (strValues map[string]interface{}, fileValues map[string]*MultipartFileData) {

	strValues = map[string]interface{}{}
	fileValues = map[string]*MultipartFileData{}

	forms := bytes.Split(body, []byte("--"+boundary))
	formCount := len(forms)
	if formCount > 1 {
		var findIndss [][]int
		var name string
		var fbean *MultipartFileData
		for _, form := range forms[1 : formCount-1] {
			if findIndss = pat_multipart_fname.FindAllSubmatchIndex(form, 1); len(findIndss) == 1 { // file

				name = string(form[findIndss[0][2]:findIndss[0][3]])
				fbean = &MultipartFileData{FileName: string(form[findIndss[0][4]:findIndss[0][5]])}
				form = form[findIndss[0][1]:]
				if findIndss = pat_multipart_ctype.FindAllSubmatchIndex(form, 1); len(findIndss) == 1 {
					fbean.ContentType = string(form[findIndss[0][2]:findIndss[0][3]])
					fbean.Content = form[findIndss[0][1]+2 : len(form)-2]
					fileValues[name] = fbean
				} else {
					log.Error("MultipartForm: Cannot find 'contenttype=xxx' at file entry: %v", string(form))
				}
			} else if findIndss = pat_multipart_name.FindAllSubmatchIndex(form, 1); len(findIndss) == 1 { // str

				strValues[string(form[findIndss[0][2]:findIndss[0][3]])] = string(form[findIndss[0][1]+2 : len(form)-2])
			} else {
				log.Error("MultipartForm: Cannot find 'filename=xxx' or 'name=xxx' at %v", string(form))
			}
		}
	}

	return
}

func URLEncode(s string) string {
	return url.QueryEscape(s)
}
func URLDecode(s string) string {
	if _s, e := url.QueryUnescape(s); e == nil {
		return _s
	} else {
		return s
	}
}

func GZipCompress(bs []byte) []byte {
	buff := bytes.NewBuffer([]byte{})
	writer := gzip.NewWriter(buff)
	writer.Write(bs)
	writer.Flush()
	writer.Close()
	bs = buff.Bytes()
	return bs
}

func GZipUncompress(bs []byte) (res []byte, e error) {
	buff := bytes.NewBuffer(bs)
	var reader *gzip.Reader
	if reader, e = gzip.NewReader(buff); e == nil {
		defer reader.Close()
		return ioutil.ReadAll(reader)
	}
	return
}
