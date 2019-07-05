package socket

import (
	"errors"
	"fmt"
	ws "github.com/gorilla/websocket"
	"net/http"

	"zwei.ren/web"
)

const (
	Text   = 1
	Binary = 2
	Close  = 8
	Ping   = 9
	Pong   = 10

	String = 1
	Bytes  = 2
)

var Err_CloseIntent = errors.New("Close by you")

type IHandler interface {
	web.IHandler
	setSelf(IHandler)

	OnConnect()
	OnDisconnect(error)
	OnMessage(msgType int, body []byte) error
}

func AddRouter(httpPath string, handlerBuilder func() IHandler) {
	AddRouterAtServer(web.DefaultServer, httpPath, handlerBuilder)
}

func AddRouterAtServer(server *web.HttpServer, httpPath string, handlerBuilder func() IHandler) {
	server.AddRouter(httpPath, func() web.IHandler {
		handler := handlerBuilder()
		handler.setSelf(handler)
		return handler
	})
}

type Handler struct {
	web.Handler

	Conn                    *ws.Conn
	headers                 http.Header
	isCloseIntent, isClosed bool
	iHandler                IHandler
}

func (this *Handler) setSelf(i IHandler) {
	this.iHandler = i
}
func (this *Handler) OnConnect() {
	fmt.Println("On Connect")
}
func (this *Handler) OnDisconnect(e error) {
	fmt.Println("Disconnect:", e)
}
func (this *Handler) OnMessage(msgType int, body []byte) error {
	return web.Err_Abort
}

func (this *Handler) Close() {
	this.isCloseIntent = true
	this.Conn.Close()
}

func (this *Handler) ResponseHeaders(headers map[string][]string) {
	if headers != nil {
		this.headers = http.Header(headers)
	}
}
func (this *Handler) Send(msgType int, body []byte) error {
	return this.Conn.WriteMessage(msgType, body)
}
func (this *Handler) dispatchDisconnect(e error) {
	if this.isClosed {
		return
	}
	this.isClosed = true
	this.iHandler.OnDisconnect(e)
}

func (this *Handler) Handle() {
	this.NoResponse()
	WebsocketUpgrader := ws.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	if conn, e := WebsocketUpgrader.Upgrade(this.Writer, this.Request, this.headers); e == nil {
		defer conn.Close()
		conn.SetCloseHandler(func(code int, text string) error {
			this.dispatchDisconnect(errors.New(fmt.Sprintf("%d-%s", code, text)))
			return nil
		})
		defer func() {
			if this.isCloseIntent {
				e = Err_CloseIntent
			} else if e == nil {
				if err := recover(); err != nil {
					switch err.(type) {
					case error:
						e = err.(error)
					default:
						e = errors.New(fmt.Sprintf("%v", err))
					}
				}
			}
			this.dispatchDisconnect(e)
		}()

		this.Conn = conn
		go func() { // 这个放在其它协程
			defer web.HandleException(this.Request.URL.Path)
			this.iHandler.OnConnect()
		}()

		var msgType int
		var msgBytes []byte

		onMsg := this.iHandler.OnMessage
		for {
			if msgType, msgBytes, e = conn.ReadMessage(); e == nil {
				e = onMsg(msgType, msgBytes)
			}
			if e != nil {
				break
			}
		}
	} else {
		this.dispatchDisconnect(e)
	}
}
