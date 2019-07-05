package session

import (
	"encoding/hex"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"zwei.ren/file"
	"zwei.ren/json"
	"zwei.ren/web"
)

var (
	Key      string        = "zwrwebid"
	Expires  time.Duration = time.Second * 3600 * 24 // 默认一天
	ValueLen int           = 8

	emptyTime   time.Time
	sessions    = map[string]*Session{}
	sessionLock = new(sync.RWMutex)

	isPersistence   bool
	persistencePath string
)

func Persistence(filePath string) {
	persistencePath = filePath
	bs, e := file.ReadFile(persistencePath, 0, -1)
	if e == nil {
		sesses := map[string]*Session{}
		if e = json.Unmarshal(bs, &sesses); e == nil {
			sessionLock.Lock()
			for k, s := range sesses {
				s.isFromPersistence = true
				sessions[k] = s
			}
			sessionLock.Unlock()
		}
	}
	sessionLock.RLock()
	bs, e = json.Marshal(sessions)
	sessionLock.RUnlock()
	if e != nil {
		panic(e)
	}
	e = file.WriteFile(filePath, bs, true, true)
	isPersistence = e == nil
}

func init() {
	rand.Seed(time.Now().Unix())
	go func() {
		span := time.Second * 60
		var curTime time.Time
		var uid string
		var sess *Session
		var delKeys []string
		for {
			time.Sleep(span)
			curTime = time.Now()
			delKeys = []string{}
			sessionLock.RLock()
			for uid, sess = range sessions {
				if sess.Expires.Before(curTime) {
					delKeys = append(delKeys, uid)
				}
			}
			sessionLock.RUnlock()

			if len(delKeys) != 0 {
				sessionLock.Lock()
				for _, uid = range delKeys {
					delete(sessions, uid)
				}
				saveSession()
				sessionLock.Unlock()
			}
		}
	}()
}

type Session struct {
	Value             interface{}
	Expires           time.Time
	cookieValue       string
	isFromPersistence bool
}

func Set(handler web.IHandler, session interface{}) {
	reader, writer := handler.GetIO()
	var cookieValue string
	if cookie, e := reader.Cookie(Key); e == nil {
		cookieValue = cookie.Value
	}
	if session == nil { // 删除session
		http.SetCookie(writer, &http.Cookie{
			Name:   Key,
			Path:   "/",
			MaxAge: -1,
		})
		if len(cookieValue) != 0 {
			sessionLock.Lock()
			defer sessionLock.Unlock()
			delete(sessions, cookieValue)
			saveSession()
		}
	} else { // 添加session
		(&Session{
			Value:       session,
			Expires:     time.Now().Add(Expires),
			cookieValue: cookieValue,
		}).Set(handler)
	}
}

func (this *Session) Set(handler web.IHandler) {
	if this == nil || this.Value == nil {
		Set(handler, nil)
	} else {
		_, writer := handler.GetIO()
		if len(this.cookieValue) == 0 {
			this.cookieValue = genCookieValue()
		}
		now := time.Now()
		if this.Expires == emptyTime {
			this.Expires = now.Add(Expires)
		}
		http.SetCookie(writer, &http.Cookie{
			Name:    Key,
			Value:   this.cookieValue,
			Path:    "/",
			Expires: this.Expires,
			MaxAge:  int(this.Expires.Unix() - now.Unix()),
		})
		sessionLock.Lock()
		defer sessionLock.Unlock()
		sessions[this.cookieValue] = this
		saveSession()
	}
}

func genCookieValue() string {
	bs := make([]byte, ValueLen)
	var value string
	var exists bool
	for {
		for i := 0; i < ValueLen; i++ {
			bs[i] = byte(rand.Int())
		}
		value = hex.EncodeToString(bs)
		sessionLock.RLock()
		_, exists = sessions[value]
		sessionLock.RUnlock()
		if !exists {
			break
		}
	}
	return value
}

func saveSession() {
	if isPersistence {
		if bs, e := json.Marshal(sessions); e == nil {
			file.WriteFile(persistencePath, bs, true, true)
		}
	}
}

// 1. 当session没有持久化的时候，直接返回内存的对象
// 2. 当session持久化时，因为不知道目标pointer，需要转换
func Get(handler web.IHandler, persistence interface{}) (cache interface{}, exists bool, isFromMemory bool) {
	reader, _ := handler.GetIO()
	if cookie, e := reader.Cookie(Key); e == nil {
		if cookieValue := cookie.Value; len(cookieValue) != 0 {
			var sess *Session
			sessionLock.RLock()
			sess, exists = sessions[cookieValue]
			sessionLock.RUnlock()
			if exists {
				if exists = sess.Value != nil && sess.Expires.After(time.Now()); exists {
					if sess.isFromPersistence { // 还没转换过
						if persistence == nil { // 接收的struct为空，不知道要转成什么类型
							exists = false
						} else { // 将默认类型(一般是interface{} > map)转成专用类型
							var bs []byte
							if bs, e = json.Marshal(sess.Value); e == nil {
								if e = json.Unmarshal(bs, persistence); e == nil {
									sess.isFromPersistence = false
									sess.Value = persistence
								}
							}
							exists = e == nil
						}
					} else { // 转换过了，直接返回成cache
						cache, isFromMemory = sess.Value, true
					}
				}
			}
		}
	}
	return
}
